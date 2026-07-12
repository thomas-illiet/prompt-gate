package faq

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"gorm.io/gorm"
)

const maxQuestionLength = 300

var compactHeadingPattern = regexp.MustCompile(`(?m)^(#{1,6})([^#\s])`)

var (
	ErrNotFound        = errors.New("faq entry not found")
	ErrInvalidID       = errors.New("invalid faq id")
	ErrQuestion        = errors.New("question is required")
	ErrAnswer          = errors.New("answer is required")
	ErrQuestionLength  = errors.New("question is too long")
	ErrInvalidPosition = errors.New("invalid position")
	ErrInvalidSort     = errors.New("invalid sort")
)

type Entry struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Question  string    `gorm:"not null" json:"question"`
	Answer    string    `gorm:"type:text;not null" json:"answer"`
	Position  int       `gorm:"not null;index" json:"position"`
	Published bool      `gorm:"not null;index" json:"published"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (e *Entry) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

type Input struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Published bool   `json:"published"`
}

type UpdateInput struct {
	Question  *string `json:"question,omitempty"`
	Answer    *string `json:"answer,omitempty"`
	Published *bool   `json:"published,omitempty"`
}

type PositionInput struct {
	Position int `json:"position"`
}

type PreviewInput struct {
	Markdown string `json:"markdown"`
}

type Response struct {
	ID           uuid.UUID `json:"id"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer"`
	RenderedHTML string    `json:"renderedHtml"`
	Position     int       `json:"position"`
	Published    bool      `json:"published"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PublicResponse struct {
	ID           uuid.UUID `json:"id"`
	Question     string    `json:"question"`
	RenderedHTML string    `json:"renderedHtml"`
	Position     int       `json:"position"`
}

type ListParams struct {
	Page, PageSize  int
	SortBy, SortDir string
}

type ListResult struct {
	Items    []Response `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int64      `json:"total"`
}

type Service struct {
	db       *gorm.DB
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
		markdown: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		),
		policy: bluemonday.UGCPolicy(),
	}
}

func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&Entry{})
}

func (s *Service) Render(markdown string) (string, error) {
	// Accept the common compact heading form (`#Title`) while retaining
	// CommonMark behavior for every other construct.
	markdown = compactHeadingPattern.ReplaceAllString(markdown, "$1 $2")
	var rendered bytes.Buffer
	if err := s.markdown.Convert([]byte(markdown), &rendered); err != nil {
		return "", fmt.Errorf("render faq markdown: %w", err)
	}
	return string(s.policy.SanitizeBytes(rendered.Bytes())), nil
}

func validate(question, answer string) (string, string, error) {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	if question == "" {
		return "", "", ErrQuestion
	}
	if len([]rune(question)) > maxQuestionLength {
		return "", "", ErrQuestionLength
	}
	if answer == "" {
		return "", "", ErrAnswer
	}
	return question, answer, nil
}

func (s *Service) response(entry Entry) (Response, error) {
	html, err := s.Render(entry.Answer)
	if err != nil {
		return Response{}, err
	}
	return Response{ID: entry.ID, Question: entry.Question, Answer: entry.Answer, RenderedHTML: html, Position: entry.Position, Published: entry.Published, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt}, nil
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrInvalidID
	}
	return id, nil
}

func (s *Service) List(ctx context.Context, params ListParams) (ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	allowed := map[string]string{"position": "position", "question": "question", "published": "published", "createdAt": "created_at", "updatedAt": "updated_at"}
	column, ok := allowed[params.SortBy]
	if !ok {
		return ListResult{}, ErrInvalidSort
	}
	dir := strings.ToLower(params.SortDir)
	if dir != "asc" && dir != "desc" {
		return ListResult{}, ErrInvalidSort
	}
	query := s.db.WithContext(ctx).Model(&Entry{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, err
	}
	var entries []Entry
	if err := query.Order(column + " " + dir).Order("id ASC").Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).Find(&entries).Error; err != nil {
		return ListResult{}, err
	}
	items := make([]Response, 0, len(entries))
	for _, entry := range entries {
		item, err := s.response(entry)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	return ListResult{Items: items, Page: params.Page, PageSize: params.PageSize, Total: total}, nil
}

func (s *Service) ListPublished(ctx context.Context) ([]PublicResponse, error) {
	var entries []Entry
	if err := s.db.WithContext(ctx).Where("published = ?", true).Order("position ASC, id ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	out := make([]PublicResponse, 0, len(entries))
	for _, entry := range entries {
		html, err := s.Render(entry.Answer)
		if err != nil {
			return nil, err
		}
		out = append(out, PublicResponse{ID: entry.ID, Question: entry.Question, RenderedHTML: html, Position: entry.Position})
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, tx *gorm.DB, rawID string) (Entry, error) {
	id, err := parseID(rawID)
	if err != nil {
		return Entry{}, err
	}
	var entry Entry
	err = tx.WithContext(ctx).First(&entry, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Entry{}, ErrNotFound
	}
	return entry, err
}

func (s *Service) Get(ctx context.Context, id string) (Response, error) {
	entry, err := s.get(ctx, s.db, id)
	if err != nil {
		return Response{}, err
	}
	return s.response(entry)
}

func (s *Service) Create(ctx context.Context, input Input) (Response, error) {
	question, answer, err := validate(input.Question, input.Answer)
	if err != nil {
		return Response{}, err
	}
	var entry Entry
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxPosition int
		if err := tx.Model(&Entry{}).Select("COALESCE(MAX(position), -1)").Scan(&maxPosition).Error; err != nil {
			return err
		}
		entry = Entry{Question: question, Answer: answer, Published: input.Published, Position: maxPosition + 1}
		return tx.Create(&entry).Error
	})
	if err != nil {
		return Response{}, err
	}
	return s.response(entry)
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Response, error) {
	entry, err := s.get(ctx, s.db, id)
	if err != nil {
		return Response{}, err
	}
	question, answer := entry.Question, entry.Answer
	if input.Question != nil {
		question = *input.Question
	}
	if input.Answer != nil {
		answer = *input.Answer
	}
	question, answer, err = validate(question, answer)
	if err != nil {
		return Response{}, err
	}
	entry.Question, entry.Answer = question, answer
	if input.Published != nil {
		entry.Published = *input.Published
	}
	if err := s.db.WithContext(ctx).Save(&entry).Error; err != nil {
		return Response{}, err
	}
	return s.response(entry)
}

func (s *Service) Move(ctx context.Context, id string, position int) (Response, error) {
	var moved Entry
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entry, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&Entry{}).Count(&count).Error; err != nil {
			return err
		}
		if position < 0 || int64(position) >= count {
			return ErrInvalidPosition
		}
		if position < entry.Position {
			if err := tx.Model(&Entry{}).Where("position >= ? AND position < ?", position, entry.Position).UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
				return err
			}
		} else if position > entry.Position {
			if err := tx.Model(&Entry{}).Where("position > ? AND position <= ?", entry.Position, position).UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
				return err
			}
		}
		entry.Position = position
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		moved = entry
		return nil
	})
	if err != nil {
		return Response{}, err
	}
	return s.response(moved)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entry, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := tx.Delete(&entry).Error; err != nil {
			return err
		}
		return tx.Model(&Entry{}).Where("position > ?", entry.Position).UpdateColumn("position", gorm.Expr("position - 1")).Error
	})
}
