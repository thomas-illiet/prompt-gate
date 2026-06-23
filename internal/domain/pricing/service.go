package pricing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"promptgate/backend/internal/domain/provider"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidPrice          = errors.New("invalid_price")
	ErrInvalidPriceTarget    = errors.New("invalid_price_target")
	ErrImmutablePriceTarget  = errors.New("immutable_price_target")
	ErrPriceProviderNotFound = errors.New("provider_not_found")
	ErrPriceNotFound         = errors.New("pricing_not_found")
	ErrPriceConflict         = errors.New("pricing_conflict")
)

type Service struct {
	db        *gorm.DB
	providers *provider.Service
}

func NewService(db *gorm.DB, providers *provider.Service) *Service {
	return &Service{db: db, providers: providers}
}

func (s *Service) AutoMigrate(ctx context.Context) error {
	if err := s.db.WithContext(ctx).AutoMigrate(&UsagePrice{}); err != nil {
		return err
	}
	if err := s.ensureGlobalFallback(ctx); err != nil {
		return err
	}
	return s.ensureConstraints(ctx)
}

func (s *Service) Config(ctx context.Context) (ConfigResponse, error) {
	fallback, err := s.globalFallback(ctx)
	if err != nil {
		return ConfigResponse{}, err
	}
	var rows []UsagePrice
	if err := s.db.WithContext(ctx).
		Where("scope = ?", ScopeModel).
		Order("provider_name ASC, model ASC").
		Find(&rows).Error; err != nil {
		return ConfigResponse{}, fmt.Errorf("list model prices: %w", err)
	}
	models := make([]ModelPriceRecord, 0, len(rows))
	for _, row := range rows {
		models = append(models, modelPriceRecord(row))
	}
	return ConfigResponse{Fallback: rates(rowPrice(fallback)), Models: models}, nil
}

func (s *Service) UpdateConfig(ctx context.Context, input UpdateConfigInput) (ConfigResponse, error) {
	if err := validateRates(input.Fallback); err != nil {
		return ConfigResponse{}, err
	}
	seen := map[string]struct{}{}
	modelRows := make([]UsagePrice, 0, len(input.Models))
	for _, item := range input.Models {
		providerName := strings.TrimSpace(item.ProviderName)
		model := strings.TrimSpace(item.Model)
		if providerName == "" || model == "" {
			return ConfigResponse{}, ErrInvalidPriceTarget
		}
		if err := s.ensureProviderNameExists(ctx, providerName); err != nil {
			return ConfigResponse{}, err
		}
		itemRates := PriceRates{Input: item.Input, Output: item.Output}
		if err := validateRates(itemRates); err != nil {
			return ConfigResponse{}, err
		}
		key := providerName + "\x00" + model
		if _, ok := seen[key]; ok {
			return ConfigResponse{}, ErrInvalidPriceTarget
		}
		seen[key] = struct{}{}
		modelRows = append(modelRows, UsagePrice{
			Scope:                ScopeModel,
			ProviderName:         providerName,
			Model:                model,
			InputUSDPer1MTokens:  item.Input,
			OutputUSDPer1MTokens: item.Output,
		})
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fallback := UsagePrice{
			ID:                   globalPriceID,
			Scope:                ScopeGlobal,
			InputUSDPer1MTokens:  input.Fallback.Input,
			OutputUSDPer1MTokens: input.Fallback.Output,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "scope"}, {Name: "provider_name"}, {Name: "model"}},
			DoUpdates: clause.AssignmentColumns([]string{"input_usd_per_1m_tokens", "output_usd_per_1m_tokens", "updated_at"}),
		}).Create(&fallback).Error; err != nil {
			return fmt.Errorf("save fallback price: %w", err)
		}
		if err := tx.Where("scope = ?", ScopeModel).Delete(&UsagePrice{}).Error; err != nil {
			return fmt.Errorf("replace model prices: %w", err)
		}
		if len(modelRows) > 0 {
			if err := tx.Create(&modelRows).Error; err != nil {
				return fmt.Errorf("save model prices: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return ConfigResponse{}, err
	}
	return s.Config(ctx)
}

func (s *Service) UpdateFallback(ctx context.Context, input PriceRates) (PriceRates, error) {
	if err := validateRates(input); err != nil {
		return PriceRates{}, err
	}
	row, err := s.globalFallback(ctx)
	if err != nil {
		return PriceRates{}, err
	}
	row.InputUSDPer1MTokens = input.Input
	row.OutputUSDPer1MTokens = input.Output
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return PriceRates{}, fmt.Errorf("update fallback price: %w", err)
	}
	return rowPrice(row), nil
}

func (s *Service) GetModelPrice(ctx context.Context, id string) (ModelPriceRecord, error) {
	row, err := s.modelPrice(ctx, id)
	if err != nil {
		return ModelPriceRecord{}, err
	}
	return modelPriceRecord(row), nil
}

func (s *Service) CreateModelPrice(ctx context.Context, input ModelPriceRecord) (ModelPriceRecord, error) {
	providerName, model, rates, err := validateModelPriceInput(input)
	if err != nil {
		return ModelPriceRecord{}, err
	}
	if err := s.ensureProviderNameExists(ctx, providerName); err != nil {
		return ModelPriceRecord{}, err
	}
	row := UsagePrice{
		Scope:                ScopeModel,
		ProviderName:         providerName,
		Model:                model,
		InputUSDPer1MTokens:  rates.Input,
		OutputUSDPer1MTokens: rates.Output,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isPriceUniqueConstraintError(err) {
			return ModelPriceRecord{}, ErrPriceConflict
		}
		return ModelPriceRecord{}, fmt.Errorf("create model price: %w", err)
	}
	return modelPriceRecord(row), nil
}

func (s *Service) UpdateModelPrice(ctx context.Context, id string, input ModelPriceRecord) (ModelPriceRecord, error) {
	providerName, model, rates, err := validateModelPriceInput(input)
	if err != nil {
		return ModelPriceRecord{}, err
	}
	row, err := s.modelPrice(ctx, id)
	if err != nil {
		return ModelPriceRecord{}, err
	}
	if modelPriceKey(row.ProviderName, row.Model) != modelPriceKey(providerName, model) {
		return ModelPriceRecord{}, ErrImmutablePriceTarget
	}
	if err := s.ensureProviderNameExists(ctx, providerName); err != nil {
		return ModelPriceRecord{}, err
	}
	row.InputUSDPer1MTokens = rates.Input
	row.OutputUSDPer1MTokens = rates.Output
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		if isPriceUniqueConstraintError(err) {
			return ModelPriceRecord{}, ErrPriceConflict
		}
		return ModelPriceRecord{}, fmt.Errorf("update model price: %w", err)
	}
	return modelPriceRecord(row), nil
}

func (s *Service) DeleteModelPrice(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Where("scope = ?", ScopeModel).Delete(&UsagePrice{}, "id = ?", strings.TrimSpace(id))
	if result.Error != nil {
		return fmt.Errorf("delete model price: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPriceNotFound
	}
	return nil
}

func (s *Service) RatesFor(ctx context.Context, providerName, model string) (PriceRates, error) {
	providerName = strings.TrimSpace(providerName)
	model = strings.TrimSpace(model)
	fallback, err := s.globalFallback(ctx)
	if err != nil {
		return PriceRates{}, err
	}
	rates := rowPrice(fallback)
	if providerName == "" || model == "" {
		return rates, nil
	}
	var row UsagePrice
	err = s.db.WithContext(ctx).
		Where("scope = ? AND provider_name = ? AND model = ?", ScopeModel, providerName, model).
		First(&row).Error
	if err == nil {
		return rowPrice(row), nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return rates, nil
	}
	return PriceRates{}, fmt.Errorf("load model price: %w", err)
}

func (s *Service) ConfigurationCheck(ctx context.Context) (ConfigurationCheckResponse, error) {
	if s.providers == nil {
		return ConfigurationCheckResponse{}, errors.New("provider service unavailable")
	}
	catalog, err := s.providers.ModelCatalog(ctx, nil)
	if err != nil {
		return ConfigurationCheckResponse{}, err
	}
	configured, err := s.configuredModelSet(ctx)
	if err != nil {
		return ConfigurationCheckResponse{}, err
	}
	missing := []MissingModelPrice{}
	providerErrors := []ProviderModelError{}
	for _, providerCatalog := range catalog {
		if providerCatalog.ModelsError != "" {
			providerErrors = append(providerErrors, ProviderModelError{ProviderName: providerCatalog.Name, Message: providerCatalog.ModelsError})
		}
		for _, model := range providerCatalog.Models {
			key := modelPriceKey(providerCatalog.Name, model)
			if _, ok := configured[key]; !ok {
				missing = append(missing, MissingModelPrice{ProviderName: providerCatalog.Name, Model: model})
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].ProviderName == missing[j].ProviderName {
			return missing[i].Model < missing[j].Model
		}
		return missing[i].ProviderName < missing[j].ProviderName
	})
	return ConfigurationCheckResponse{
		Configured:     len(missing) == 0 && len(providerErrors) == 0,
		MissingPrices:  missing,
		ProviderErrors: providerErrors,
		CheckedAt:      time.Now().UTC(),
	}, nil
}

func (s *Service) ensureGlobalFallback(ctx context.Context) error {
	row := UsagePrice{ID: globalPriceID, Scope: ScopeGlobal, InputUSDPer1MTokens: 5, OutputUSDPer1MTokens: 30}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (s *Service) ensureConstraints(ctx context.Context) error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	queries := []string{
		`ALTER TABLE usage_prices ADD CONSTRAINT usage_prices_scope_check CHECK (scope IN ('global', 'model'))`,
		`ALTER TABLE usage_prices ADD CONSTRAINT usage_prices_shape_check CHECK ((scope = 'global' AND provider_name = '' AND model = '') OR (scope = 'model' AND provider_name <> '' AND model <> ''))`,
	}
	for _, query := range queries {
		if err := s.db.WithContext(ctx).Exec(query).Error; err != nil && !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	return nil
}

func (s *Service) globalFallback(ctx context.Context) (UsagePrice, error) {
	var row UsagePrice
	if err := s.db.WithContext(ctx).
		Where("scope = ? AND provider_name = '' AND model = ''", ScopeGlobal).
		First(&row).Error; err != nil {
		return UsagePrice{}, fmt.Errorf("load fallback price: %w", err)
	}
	return row, nil
}

func (s *Service) configuredModelSet(ctx context.Context) (map[string]struct{}, error) {
	var rows []UsagePrice
	if err := s.db.WithContext(ctx).Where("scope = ?", ScopeModel).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list configured model prices: %w", err)
	}
	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		out[modelPriceKey(row.ProviderName, row.Model)] = struct{}{}
	}
	return out, nil
}

func validateRates(r PriceRates) error {
	if r.Input < 0 || r.Output < 0 {
		return ErrInvalidPrice
	}
	return nil
}

func validateModelPriceInput(input ModelPriceRecord) (string, string, PriceRates, error) {
	providerName := strings.TrimSpace(input.ProviderName)
	model := strings.TrimSpace(input.Model)
	if providerName == "" || model == "" {
		return "", "", PriceRates{}, ErrInvalidPriceTarget
	}
	rates := PriceRates{Input: input.Input, Output: input.Output}
	if err := validateRates(rates); err != nil {
		return "", "", PriceRates{}, err
	}
	return providerName, model, rates, nil
}

func (s *Service) ensureProviderNameExists(ctx context.Context, providerName string) error {
	if s.providers == nil {
		return nil
	}
	exists, err := s.providers.ProviderNameExists(ctx, providerName)
	if err != nil {
		return err
	}
	if !exists {
		return ErrPriceProviderNotFound
	}
	return nil
}

func (s *Service) modelPrice(ctx context.Context, id string) (UsagePrice, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return UsagePrice{}, ErrPriceNotFound
	}
	var row UsagePrice
	err := s.db.WithContext(ctx).
		Where("scope = ? AND id = ?", ScopeModel, id).
		First(&row).Error
	if err == nil {
		return row, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UsagePrice{}, ErrPriceNotFound
	}
	return UsagePrice{}, fmt.Errorf("load model price: %w", err)
}

func isPriceUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}

func modelPriceRecord(row UsagePrice) ModelPriceRecord {
	return ModelPriceRecord{
		ID:           row.ID,
		ProviderName: row.ProviderName,
		Model:        row.Model,
		Input:        row.InputUSDPer1MTokens,
		Output:       row.OutputUSDPer1MTokens,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func rowPrice(row UsagePrice) PriceRates {
	return PriceRates{Input: row.InputUSDPer1MTokens, Output: row.OutputUSDPer1MTokens}
}

func rates(r PriceRates) PriceRates { return r }

func modelPriceKey(providerName, model string) string {
	return strings.TrimSpace(providerName) + "\x00" + strings.TrimSpace(model)
}
