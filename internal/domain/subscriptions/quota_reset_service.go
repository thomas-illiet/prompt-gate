package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	quotaResetReconcileBatchSize  = 100
	quotaResetReconcileMaxBatches = 10
)

// QuotaResetSummary returns the durable reset balance and current applicability for a human user.
func (s *Service) QuotaResetSummary(ctx context.Context, userID string, _ time.Time) (QuotaResetSummary, error) {
	var summary QuotaResetSummary
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := quotaResetUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		_, hasResettableQuota, err := effectiveQuotaResetPlan(ctx, tx, account)
		if err != nil {
			return err
		}
		summary, err = quotaResetSummaryForUser(ctx, tx, account.ID, hasResettableQuota)
		return err
	})
	return summary, err
}

// ListQuotaResetHistory returns one user's reset balance ledger in reverse chronological order.
func (s *Service) ListQuotaResetHistory(ctx context.Context, userID string, params QuotaResetHistoryParams) (QuotaResetHistoryResult, error) {
	if _, err := quotaResetUser(ctx, s.db, userID); err != nil {
		return QuotaResetHistoryResult{}, err
	}
	normalizeQuotaResetHistoryParams(&params)
	query := s.db.WithContext(ctx).
		Model(&QuotaResetEvent{}).
		Where("user_id = ? AND status = ?", strings.TrimSpace(userID), QuotaResetEventApplied)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return QuotaResetHistoryResult{}, fmt.Errorf("count quota reset history: %w", err)
	}
	var records []QuotaResetEvent
	if err := query.
		Order("created_at DESC").
		Order("id DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return QuotaResetHistoryResult{}, fmt.Errorf("list quota reset history: %w", err)
	}
	items := make([]QuotaResetHistoryItem, 0, len(records))
	for _, record := range records {
		items = append(items, quotaResetHistoryItem(record))
	}
	return QuotaResetHistoryResult{
		Items: items, Page: params.Page, PageSize: params.PageSize, Total: total,
	}, nil
}

// GrantQuotaResetCredit grants exactly one reset credit to a human user.
// The idempotency key is hashed before persistence and replay returns the current summary.
func (s *Service) GrantQuotaResetCredit(
	ctx context.Context,
	userID string,
	actor QuotaResetActor,
	idempotencyKey string,
	now time.Time,
) (QuotaResetSummary, error) {
	hashedKey, err := normalizeQuotaResetIdempotencyKey(idempotencyKey)
	if err != nil {
		return QuotaResetSummary{}, err
	}
	actor, err = normalizeQuotaResetActor(actor)
	if err != nil {
		return QuotaResetSummary{}, err
	}
	now = quotaResetNow(now)

	var summary QuotaResetSummary
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := quotaResetUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		_, hasResettableQuota, err := effectiveQuotaResetPlan(ctx, tx, account)
		if err != nil {
			return err
		}
		wallet, err := lockQuotaResetWallet(ctx, tx, account.ID)
		if err != nil {
			return err
		}

		var existing QuotaResetEvent
		err = tx.WithContext(ctx).
			Where("user_id = ? AND type = ? AND idempotency_key = ?", account.ID, QuotaResetEventAdminGrant, *hashedKey).
			Take(&existing).Error
		if err == nil {
			summary, err = quotaResetSummaryWithWallet(ctx, tx, wallet, hasResettableQuota)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load idempotent quota reset grant: %w", err)
		}
		if wallet.Available >= MaxQuotaResetCredits {
			return ErrQuotaResetBalanceFull
		}

		wallet.Available++
		if err := tx.WithContext(ctx).Save(&wallet).Error; err != nil {
			return fmt.Errorf("update quota reset wallet: %w", err)
		}
		appliedAt := now
		event := QuotaResetEvent{
			UserID:         account.ID,
			Type:           QuotaResetEventAdminGrant,
			Status:         QuotaResetEventApplied,
			Delta:          1,
			BalanceAfter:   wallet.Available,
			ActorType:      actor.Type,
			ActorUserID:    cloneStringPtr(actor.UserID),
			IdempotencyKey: hashedKey,
			AppliedAt:      &appliedAt,
			CreatedAt:      now,
		}
		if err := tx.WithContext(ctx).Create(&event).Error; err != nil {
			return fmt.Errorf("record quota reset grant: %w", err)
		}
		summary, err = quotaResetSummaryWithWallet(ctx, tx, wallet, hasResettableQuota)
		return err
	})
	return summary, err
}

// UseQuotaReset reserves one credit under a PostgreSQL row lock, then applies
// an idempotent Redis cutoff. Operational Redis failures leave a durable pending
// event for the scheduler to reconcile instead of losing the credit or request.
func (s *Service) UseQuotaReset(
	ctx context.Context,
	store *RedisStore,
	userID string,
	idempotencyKey string,
	now time.Time,
) (QuotaResetUseResult, error) {
	hashedKey, err := normalizeQuotaResetIdempotencyKey(idempotencyKey)
	if err != nil {
		return QuotaResetUseResult{}, err
	}
	now = quotaResetNow(now)

	reservation, err := s.reserveQuotaReset(ctx, userID, hashedKey, now)
	if err != nil {
		return QuotaResetUseResult{}, err
	}
	result := QuotaResetUseResult{
		Status:  QuotaResetUsePending,
		Summary: reservation.summary,
	}
	if reservation.event.Status == QuotaResetEventApplied {
		if store == nil || !store.Enabled() {
			return QuotaResetUseResult{}, errors.New("subscription quota store unavailable")
		}
		result.Status = QuotaResetUseApplied
		if refreshErr := store.RefreshIfStale(ctx); refreshErr != nil {
			return QuotaResetUseResult{}, fmt.Errorf("refresh subscription quota snapshot for reset replay: %w", refreshErr)
		}
		quota, quotaErr := store.CurrentQuota(ctx, userID, now)
		if quotaErr != nil {
			return QuotaResetUseResult{}, fmt.Errorf("load live quota for reset replay: %w", quotaErr)
		}
		result.Quota = &quota
		return result, nil
	}

	application, applyErr := s.applyPendingQuotaReset(ctx, store, reservation.event, now)
	if applyErr != nil || !application.applied {
		return result, nil
	}
	result.Status = QuotaResetUseApplied
	result.Quota = &application.quota
	if refreshed, refreshErr := s.QuotaResetSummary(ctx, userID, now); refreshErr == nil {
		result.Summary = refreshed
	} else {
		result.Summary.PendingReset = false
	}
	return result, nil
}

// ReconcilePendingQuotaResets applies durable pending reset reservations in fair,
// bounded batches. Failed records gain an attempt count so a permanently invalid
// record cannot monopolize the oldest page and starve newer reservations.
func (s *Service) ReconcilePendingQuotaResets(ctx context.Context, store *RedisStore, now time.Time) (int64, error) {
	if store == nil || !store.Enabled() {
		return 0, errors.New("subscription quota store unavailable")
	}
	now = quotaResetNow(now)
	var applied int64
	var reconcileErr error
	attemptedIDs := make([]string, 0, quotaResetReconcileBatchSize*quotaResetReconcileMaxBatches)
	for batch := 0; batch < quotaResetReconcileMaxBatches; batch++ {
		var pending []QuotaResetEvent
		query := s.db.WithContext(ctx).
			Where("type = ? AND status = ?", QuotaResetEventUsed, QuotaResetEventPending)
		if len(attemptedIDs) > 0 {
			query = query.Where("id NOT IN ?", attemptedIDs)
		}
		if err := query.
			Order("reconcile_attempts ASC").
			Order("created_at ASC").
			Order("id ASC").
			Limit(quotaResetReconcileBatchSize).
			Find(&pending).Error; err != nil {
			return applied, errors.Join(reconcileErr, fmt.Errorf("list pending quota resets: %w", err))
		}
		if len(pending) == 0 {
			break
		}

		for _, event := range pending {
			attemptedIDs = append(attemptedIDs, event.ID)
			application, err := s.applyPendingQuotaReset(ctx, store, event, now)
			if err != nil {
				if recordErr := s.recordQuotaResetReconcileFailure(ctx, event.ID, now); recordErr != nil {
					err = errors.Join(err, recordErr)
				}
				reconcileErr = errors.Join(reconcileErr, fmt.Errorf("reconcile quota reset %s: %w", event.ID, err))
				continue
			}
			if application.changed {
				applied++
			}
		}
		if len(pending) < quotaResetReconcileBatchSize {
			break
		}
	}
	return applied, reconcileErr
}

// DistributeMonthlyQuotaResetCredits grants one credit per UTC calendar month
// to active human users with an effective finite quota. A month is marked as
// processed even at the two-credit cap so a consumed credit is not backfilled.
func (s *Service) DistributeMonthlyQuotaResetCredits(ctx context.Context, now time.Time) (QuotaResetDistributionResult, error) {
	now = quotaResetNow(now)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var userIDs []string
	if err := s.db.WithContext(ctx).
		Model(&users.User{}).
		Where("type = ? AND is_active = ? AND role IN ?", auth.UserTypeUser, true, []auth.AppRole{auth.RoleLimited, auth.RoleUser, auth.RoleAdmin}).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("id ASC").
		Pluck("id", &userIDs).Error; err != nil {
		return QuotaResetDistributionResult{}, fmt.Errorf("list monthly quota reset users: %w", err)
	}

	var result QuotaResetDistributionResult
	for _, userID := range userIDs {
		outcome, err := s.distributeMonthlyQuotaResetCredit(ctx, userID, month, now)
		if err != nil {
			return result, err
		}
		switch outcome {
		case monthlyQuotaResetIneligible:
			continue
		case monthlyQuotaResetGranted:
			result.Eligible++
			result.Granted++
		case monthlyQuotaResetAlreadyProcessed:
			result.Eligible++
			result.AlreadyProcessed++
		case monthlyQuotaResetAtMaximum:
			result.Eligible++
			result.SkippedAtMaximum++
		}
	}
	return result, nil
}

type quotaResetReservation struct {
	event   QuotaResetEvent
	summary QuotaResetSummary
}

func (s *Service) reserveQuotaReset(ctx context.Context, userID string, hashedKey *string, now time.Time) (quotaResetReservation, error) {
	var reservation quotaResetReservation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := quotaResetUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		wallet, err := lockQuotaResetWallet(ctx, tx, account.ID)
		if err != nil {
			return err
		}
		_, hasResettableQuota, err := effectiveQuotaResetPlan(ctx, tx, account)
		if err != nil {
			return err
		}
		var existing QuotaResetEvent
		err = tx.WithContext(ctx).
			Where("user_id = ? AND type = ? AND idempotency_key = ?", account.ID, QuotaResetEventUsed, *hashedKey).
			Take(&existing).Error
		if err == nil {
			reservation.event = existing
			reservation.summary, err = quotaResetSummaryWithWallet(ctx, tx, wallet, hasResettableQuota)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load idempotent quota reset use: %w", err)
		}
		if !hasResettableQuota {
			return ErrQuotaResetNotApplicable
		}

		var pending int64
		if err := tx.WithContext(ctx).Model(&QuotaResetEvent{}).
			Where("user_id = ? AND type = ? AND status = ?", account.ID, QuotaResetEventUsed, QuotaResetEventPending).
			Count(&pending).Error; err != nil {
			return fmt.Errorf("count pending quota resets: %w", err)
		}
		if pending > 0 {
			return ErrQuotaResetPending
		}
		if wallet.Available <= 0 {
			return ErrQuotaResetUnavailable
		}

		wallet.Available--
		wallet.ResetSequence++
		if err := tx.WithContext(ctx).Save(&wallet).Error; err != nil {
			return fmt.Errorf("reserve quota reset credit: %w", err)
		}
		sequence := wallet.ResetSequence
		actorUserID := account.ID
		event := QuotaResetEvent{
			UserID:         account.ID,
			Type:           QuotaResetEventUsed,
			Status:         QuotaResetEventPending,
			Delta:          -1,
			BalanceAfter:   wallet.Available,
			ActorType:      QuotaResetActorUser,
			ActorUserID:    &actorUserID,
			IdempotencyKey: hashedKey,
			ResetSequence:  &sequence,
			CreatedAt:      now,
		}
		if err := tx.WithContext(ctx).Create(&event).Error; err != nil {
			return fmt.Errorf("record pending quota reset: %w", err)
		}
		reservation.event = event
		reservation.summary, err = quotaResetSummaryWithWallet(ctx, tx, wallet, hasResettableQuota)
		return err
	})
	return reservation, err
}

type pendingQuotaResetApplication struct {
	quota   QuotaStatus
	applied bool
	changed bool
}

func (s *Service) applyPendingQuotaReset(
	ctx context.Context,
	store *RedisStore,
	event QuotaResetEvent,
	now time.Time,
) (pendingQuotaResetApplication, error) {
	if event.Status == QuotaResetEventApplied {
		if store == nil || !store.Enabled() {
			return pendingQuotaResetApplication{applied: true}, nil
		}
		quota, err := store.CurrentQuota(ctx, event.UserID, now)
		return pendingQuotaResetApplication{quota: quota, applied: true}, err
	}
	if event.ResetSequence == nil || *event.ResetSequence <= 0 {
		return pendingQuotaResetApplication{}, errors.New("pending quota reset has no sequence")
	}
	if store == nil || !store.Enabled() {
		return pendingQuotaResetApplication{}, errors.New("subscription quota store unavailable")
	}
	if err := store.RefreshIfStale(ctx); err != nil {
		return pendingQuotaResetApplication{}, fmt.Errorf("refresh subscription quota snapshot before reset: %w", err)
	}

	application, err := store.ApplyQuotaReset(ctx, event.UserID, *event.ResetSequence)
	if err != nil {
		return pendingQuotaResetApplication{}, err
	}
	quota, observedAt, err := store.currentQuota(ctx, event.UserID, time.Now().UTC())
	if err != nil {
		return pendingQuotaResetApplication{}, fmt.Errorf("load quota after reset: %w", err)
	}

	result := pendingQuotaResetApplication{quota: quota}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current QuotaResetEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", event.ID).
			Take(&current).Error; err != nil {
			return fmt.Errorf("lock pending quota reset: %w", err)
		}
		if current.Status == QuotaResetEventApplied {
			result.applied = true
			return nil
		}
		current.Status = QuotaResetEventApplied
		current.ResetCutoffAt = &application.Cutoff
		current.AppliedAt = &observedAt
		if err := tx.WithContext(ctx).Save(&current).Error; err != nil {
			return fmt.Errorf("mark quota reset applied: %w", err)
		}
		if err := s.upsertQuotaState(ctx, tx, current.UserID, quota, observedAt); err != nil {
			return err
		}
		result.applied = true
		result.changed = true
		return nil
	})
	if err != nil {
		return pendingQuotaResetApplication{}, err
	}
	return result, nil
}

func (s *Service) recordQuotaResetReconcileFailure(ctx context.Context, eventID string, attemptedAt time.Time) error {
	result := s.db.WithContext(ctx).Model(&QuotaResetEvent{}).
		Where("id = ? AND type = ? AND status = ?", eventID, QuotaResetEventUsed, QuotaResetEventPending).
		Updates(map[string]any{
			"reconcile_attempts":        gorm.Expr("reconcile_attempts + 1"),
			"last_reconcile_attempt_at": attemptedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("record quota reset reconciliation failure: %w", result.Error)
	}
	return nil
}

type monthlyQuotaResetOutcome int

const (
	monthlyQuotaResetIneligible monthlyQuotaResetOutcome = iota
	monthlyQuotaResetGranted
	monthlyQuotaResetAlreadyProcessed
	monthlyQuotaResetAtMaximum
)

func (s *Service) distributeMonthlyQuotaResetCredit(
	ctx context.Context,
	userID string,
	month time.Time,
	now time.Time,
) (monthlyQuotaResetOutcome, error) {
	outcome := monthlyQuotaResetIneligible
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := lockMonthlyQuotaResetUser(ctx, tx, userID, now)
		if err != nil {
			if errors.Is(err, ErrQuotaResetUserNotFound) {
				return nil
			}
			return err
		}
		_, eligible, err := effectiveQuotaResetPlan(ctx, tx, account)
		if err != nil {
			return err
		}
		if !eligible {
			return nil
		}
		wallet, err := lockQuotaResetWallet(ctx, tx, account.ID)
		if err != nil {
			return err
		}
		if sameUTCMonth(wallet.LastMonthlyGrantMonth, month) {
			outcome = monthlyQuotaResetAlreadyProcessed
			return nil
		}
		wallet.LastMonthlyGrantMonth = &month
		if wallet.Available >= MaxQuotaResetCredits {
			if err := tx.WithContext(ctx).Save(&wallet).Error; err != nil {
				return fmt.Errorf("mark monthly quota reset processed: %w", err)
			}
			outcome = monthlyQuotaResetAtMaximum
			return nil
		}

		wallet.Available++
		if err := tx.WithContext(ctx).Save(&wallet).Error; err != nil {
			return fmt.Errorf("grant monthly quota reset: %w", err)
		}
		appliedAt := now
		event := QuotaResetEvent{
			UserID:       account.ID,
			Type:         QuotaResetEventMonthlyGrant,
			Status:       QuotaResetEventApplied,
			Delta:        1,
			BalanceAfter: wallet.Available,
			ActorType:    QuotaResetActorSystem,
			GrantMonth:   &month,
			AppliedAt:    &appliedAt,
			CreatedAt:    now,
		}
		if err := tx.WithContext(ctx).Create(&event).Error; err != nil {
			return fmt.Errorf("record monthly quota reset grant: %w", err)
		}
		outcome = monthlyQuotaResetGranted
		return nil
	})
	return outcome, err
}

func lockMonthlyQuotaResetUser(ctx context.Context, tx *gorm.DB, userID string, now time.Time) (users.User, error) {
	var account users.User
	err := tx.WithContext(ctx).
		Where("id = ? AND type = ? AND is_active = ? AND role <> ?", strings.TrimSpace(userID), auth.UserTypeUser, true, auth.RoleNone).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Take(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return users.User{}, ErrQuotaResetUserNotFound
	}
	if err != nil {
		return users.User{}, fmt.Errorf("lock monthly quota reset user: %w", err)
	}
	return account, nil
}

func quotaResetUser(ctx context.Context, tx *gorm.DB, userID string) (users.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return users.User{}, ErrQuotaResetUserNotFound
	}
	var account users.User
	if err := tx.WithContext(ctx).
		Where("id = ? AND type = ?", userID, auth.UserTypeUser).
		Take(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return users.User{}, ErrQuotaResetUserNotFound
		}
		return users.User{}, fmt.Errorf("load quota reset user: %w", err)
	}
	return account, nil
}

func effectiveQuotaResetPlan(ctx context.Context, tx *gorm.DB, account users.User) (SubscriptionPlan, bool, error) {
	query := tx.WithContext(ctx).Model(&SubscriptionPlan{})
	if account.SubscriptionPlanID != nil {
		query = query.Where("id = ?", *account.SubscriptionPlanID)
	} else {
		query = query.Where("is_default = ?", true)
	}
	var plan SubscriptionPlan
	if err := query.Take(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SubscriptionPlan{}, false, nil
		}
		return SubscriptionPlan{}, false, fmt.Errorf("load effective quota reset plan: %w", err)
	}
	return plan, plan.Quota5HTokens != nil || plan.Quota7DTokens != nil, nil
}

func lockQuotaResetWallet(ctx context.Context, tx *gorm.DB, userID string) (QuotaResetWallet, error) {
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&QuotaResetWallet{UserID: userID}).Error; err != nil {
		return QuotaResetWallet{}, fmt.Errorf("ensure quota reset wallet: %w", err)
	}
	var wallet QuotaResetWallet
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		Take(&wallet).Error; err != nil {
		return QuotaResetWallet{}, fmt.Errorf("lock quota reset wallet: %w", err)
	}
	return wallet, nil
}

func quotaResetSummaryForUser(ctx context.Context, tx *gorm.DB, userID string, hasResettableQuota bool) (QuotaResetSummary, error) {
	var wallet QuotaResetWallet
	err := tx.WithContext(ctx).Where("user_id = ?", userID).Take(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		wallet.UserID = userID
	} else if err != nil {
		return QuotaResetSummary{}, fmt.Errorf("load quota reset wallet: %w", err)
	}
	return quotaResetSummaryWithWallet(ctx, tx, wallet, hasResettableQuota)
}

func quotaResetSummaryWithWallet(ctx context.Context, tx *gorm.DB, wallet QuotaResetWallet, hasResettableQuota bool) (QuotaResetSummary, error) {
	var pending int64
	if err := tx.WithContext(ctx).Model(&QuotaResetEvent{}).
		Where("user_id = ? AND type = ? AND status = ?", wallet.UserID, QuotaResetEventUsed, QuotaResetEventPending).
		Count(&pending).Error; err != nil {
		return QuotaResetSummary{}, fmt.Errorf("count pending quota resets: %w", err)
	}
	return QuotaResetSummary{
		Available:          wallet.Available,
		Maximum:            MaxQuotaResetCredits,
		HasResettableQuota: hasResettableQuota,
		PendingReset:       pending > 0,
	}, nil
}

func normalizeQuotaResetHistoryParams(params *QuotaResetHistoryParams) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
}

func normalizeQuotaResetActor(actor QuotaResetActor) (QuotaResetActor, error) {
	if actor.UserID != nil {
		value := strings.TrimSpace(*actor.UserID)
		if value == "" {
			actor.UserID = nil
		} else {
			actor.UserID = &value
		}
	}
	if actor.Type == "" {
		if actor.UserID == nil {
			actor.Type = QuotaResetActorAdminAPIKey
		} else {
			actor.Type = QuotaResetActorAdminSession
		}
	}
	switch actor.Type {
	case QuotaResetActorAdminSession:
		if actor.UserID == nil {
			return QuotaResetActor{}, errors.New("admin session quota reset actor requires a user id")
		}
	case QuotaResetActorAdminAPIKey:
		if actor.UserID != nil {
			return QuotaResetActor{}, errors.New("admin api key quota reset actor cannot have a user id")
		}
	default:
		return QuotaResetActor{}, errors.New("invalid administrator quota reset actor")
	}
	return actor, nil
}

func quotaResetNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func sameUTCMonth(value *time.Time, month time.Time) bool {
	if value == nil {
		return false
	}
	utc := value.UTC()
	return utc.Year() == month.Year() && utc.Month() == month.Month()
}
