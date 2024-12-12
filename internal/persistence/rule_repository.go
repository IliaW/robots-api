package persistence

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/IliaW/robots-api/internal/model"
	"github.com/IliaW/robots-api/util"
)

//go:generate go run github.com/vektra/mockery/v2@v2.50.0 --name RuleStorage
type RuleStorage interface {
	GetByUrl(string) (*model.Rule, error)
	GetById(string) (*model.Rule, error)
	Save(*model.Rule) (int64, error)
	Update(*model.Rule) (*model.Rule, error)
	Delete(string) error
}

type RuleRepository struct {
	db  *sql.DB
	log *slog.Logger
	mu  sync.Mutex
}

func NewRuleRepository(db *sql.DB, log *slog.Logger) *RuleRepository {
	return &RuleRepository{
		db:  db,
		log: log,
	}
}

func (r *RuleRepository) GetByUrl(url string) (*model.Rule, error) {
	domain, err := util.GetDomain(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to parse url. %s", err.Error()))
	}
	var rule model.Rule
	row := r.db.QueryRow("SELECT id, domain, robots_txt, created_at, updated_at FROM custom_rule WHERE domain = ?",
		domain)
	err = row.Scan(&rule.ID, &rule.Domain, &rule.RobotsTxt, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(fmt.Sprintf("rule with domain '%s' not found", domain))
		}
		r.log.Debug("failed to get rule from database.", slog.String("err", err.Error()))
		return nil, err
	}
	r.log.Debug("rule fetched from db.")

	return &rule, nil
}

func (r *RuleRepository) GetById(id string) (*model.Rule, error) {
	var rule model.Rule
	row := r.db.QueryRow("SELECT id, domain, robots_txt, created_at, updated_at FROM custom_rule WHERE id = ?",
		id)
	err := row.Scan(&rule.ID, &rule.Domain, &rule.RobotsTxt, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(fmt.Sprintf("rule with id '%s' not found", id))
		}
		r.log.Debug("failed to get rule from database.", slog.String("err", err.Error()))
		return nil, err
	}
	r.log.Debug("rule fetched from db.")

	return &rule, nil
}

func (r *RuleRepository) Save(rule *model.Rule) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result, err := r.db.Exec("INSERT INTO custom_rule (domain, robots_txt) VALUES (?, ?)",
		rule.Domain, rule.RobotsTxt)
	if err != nil {
		return 0, err
	}
	r.log.Debug("rule saved to db.")

	return result.LastInsertId()
}

func (r *RuleRepository) Update(rule *model.Rule) (*model.Rule, error) {
	_, err := r.db.Exec("UPDATE custom_rule SET domain = ?, robots_txt = ? WHERE id = ?",
		rule.Domain, rule.RobotsTxt, rule.ID)
	if err != nil {
		return nil, err
	}
	r.log.Debug("rule updated in db.")

	return r.GetById(strconv.Itoa(rule.ID))
}

func (r *RuleRepository) Delete(ruleId string) error {
	_, err := r.db.Exec("DELETE FROM custom_rule WHERE id = ?", ruleId)
	if err != nil {
		return err
	}
	r.log.Debug("rule deleted from db.")

	return nil
}
