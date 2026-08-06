package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/linguaquest/server/internal/domain"
)

func (s *PostgresStore) CreatePaymentOrder(order domain.PaymentOrder) (domain.PaymentOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO payment_orders (id, user_id, product_code, amount_cents, payment_channel, status, created_at) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)`, order.ID, order.UserID, order.ProductCode, order.AmountCents, order.PaymentChannel, order.Status, order.CreatedAt)
	return order, err
}

func (s *PostgresStore) GetPaymentOrder(orderID string, userID string) (domain.PaymentOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `SELECT id, user_id::text, product_code, amount_cents, payment_channel, status, provider_trade_no, created_at, paid_at FROM payment_orders WHERE id = $1`
	args := []any{orderID}
	if strings.TrimSpace(userID) != "" {
		query += ` AND user_id = $2::uuid`
		args = append(args, userID)
	}
	return scanPostgresPaymentOrder(s.pool.QueryRow(ctx, query, args...))
}

func (s *PostgresStore) MarkPaymentOrderPaid(orderID string, providerTradeNo string, product domain.BillingProduct, paidAt time.Time) (domain.PaymentOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	defer tx.Rollback(ctx)
	order, err := scanPostgresPaymentOrder(tx.QueryRow(ctx, `SELECT id, user_id::text, product_code, amount_cents, payment_channel, status, provider_trade_no, created_at, paid_at FROM payment_orders WHERE id = $1 FOR UPDATE`, orderID))
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	if order.Status == "PAID" {
		return order, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE payment_orders SET status = 'PAID', provider_trade_no = $2, paid_at = $3 WHERE id = $1`, orderID, providerTradeNo, paidAt); err != nil {
		return domain.PaymentOrder{}, err
	}
	var currentLifetime bool
	err = tx.QueryRow(ctx, `SELECT is_lifetime FROM billing_entitlements WHERE user_id = $1::uuid FOR UPDATE`, order.UserID).Scan(&currentLifetime)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.PaymentOrder{}, err
	}
	if currentLifetime {
		_, err = tx.Exec(ctx, `UPDATE billing_entitlements SET credit_balance = credit_balance + $2, updated_at = $3 WHERE user_id = $1::uuid`, order.UserID, product.CreditAllowance, paidAt)
	} else {
		isLifetime := product.Kind == "LIFETIME"
		var expiresAt *time.Time
		if !isLifetime {
			value := paidAt.AddDate(0, 0, product.PeriodDays)
			expiresAt = &value
		}
		_, err = tx.Exec(ctx, `INSERT INTO billing_entitlements (user_id, product_code, product_name, is_lifetime, ads_free, credit_balance, credit_allowance, credit_reset_at, expires_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT(user_id) DO UPDATE SET product_code=EXCLUDED.product_code, product_name=EXCLUDED.product_name, is_lifetime=EXCLUDED.is_lifetime, ads_free=EXCLUDED.ads_free, credit_balance=EXCLUDED.credit_balance, credit_allowance=EXCLUDED.credit_allowance, credit_reset_at=EXCLUDED.credit_reset_at, expires_at=EXCLUDED.expires_at, updated_at=EXCLUDED.updated_at`,
			order.UserID, product.Code, product.Name, isLifetime, product.AdsFree, product.CreditAllowance, product.CreditAllowance, paidAt.AddDate(0, 0, product.PeriodDays), expiresAt, paidAt)
	}
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.PaymentOrder{}, err
	}
	order.Status = "PAID"
	order.ProviderTradeNo = providerTradeNo
	order.PaidAt = paidAt
	return order, nil
}

func (s *PostgresStore) GetBillingStatus(userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var status domain.BillingStatus
	var creditResetAt, expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `SELECT product_code, product_name, is_lifetime, ads_free, credit_balance, credit_allowance, credit_reset_at, expires_at FROM billing_entitlements WHERE user_id = $1::uuid`, userID).Scan(&status.ProductCode, &status.ProductName, &status.IsLifetime, &status.AdsFree, &status.CreditBalance, &status.CreditAllowance, &creditResetAt, &expiresAt)
	if err == nil {
		if creditResetAt != nil {
			status.CreditResetAt = *creditResetAt
		}
		if expiresAt != nil {
			status.ExpiresAt = *expiresAt
		}
		if status.IsLifetime && !status.CreditResetAt.After(now) {
			status.CreditBalance = status.CreditAllowance
			status.CreditResetAt = now.AddDate(0, 0, 30)
			if _, err = s.pool.Exec(ctx, `UPDATE billing_entitlements SET credit_balance=$2, credit_reset_at=$3, updated_at=$4 WHERE user_id=$1::uuid`, userID, status.CreditBalance, status.CreditResetAt, now); err != nil {
				return domain.BillingStatus{}, err
			}
		}
		if status.IsLifetime || status.ExpiresAt.After(now) {
			return status, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return domain.BillingStatus{}, err
	}
	return s.postgresFreeBillingStatus(ctx, userID, now, freeDailyCredits)
}

func (s *PostgresStore) ConsumeCredits(userID string, activity string, sourceID string, amount int, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	status, err := s.GetBillingStatus(userID, now, freeDailyCredits)
	if err != nil {
		return domain.BillingStatus{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.BillingStatus{}, err
	}
	defer tx.Rollback(ctx)
	var duplicate int
	err = tx.QueryRow(ctx, `SELECT 1 FROM credit_usages WHERE user_id=$1::uuid AND activity=$2 AND source_id=$3`, userID, activity, sourceID).Scan(&duplicate)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return domain.BillingStatus{}, err
		}
		return s.GetBillingStatus(userID, now, freeDailyCredits)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.BillingStatus{}, err
	}
	isFree := status.ProductCode == "free"
	if isFree {
		freeStatus, statusErr := s.postgresFreeBillingStatusTx(ctx, tx, userID, now, freeDailyCredits)
		if statusErr != nil {
			return domain.BillingStatus{}, statusErr
		}
		if freeStatus.CreditBalance < amount {
			return freeStatus, errors.New("AI 点数不足，请等待每日点数重置")
		}
	} else {
		result, updateErr := tx.Exec(ctx, `UPDATE billing_entitlements SET credit_balance=credit_balance-$2, updated_at=$3 WHERE user_id=$1::uuid AND credit_balance >= $2`, userID, amount, now)
		if updateErr != nil {
			return domain.BillingStatus{}, updateErr
		}
		if result.RowsAffected() == 0 {
			return status, errors.New("AI 点数不足，请等待每日点数重置")
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO credit_usages (id, user_id, activity, source_id, amount, is_free, created_at) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)`, uuid.NewString(), userID, activity, sourceID, amount, isFree, now)
	if err != nil {
		return domain.BillingStatus{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.BillingStatus{}, err
	}
	return s.GetBillingStatus(userID, now, freeDailyCredits)
}

func (s *PostgresStore) RefundCredits(userID string, activity string, sourceID string, amount int, now time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var isFree bool
	err = tx.QueryRow(ctx, `SELECT is_free FROM credit_usages WHERE user_id=$1::uuid AND activity=$2 AND source_id=$3 FOR UPDATE`, userID, activity, sourceID).Scan(&isFree)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM credit_usages WHERE user_id=$1::uuid AND activity=$2 AND source_id=$3`, userID, activity, sourceID); err != nil {
		return err
	}
	if !isFree {
		if _, err = tx.Exec(ctx, `UPDATE billing_entitlements SET credit_balance=credit_balance+$2, updated_at=$3 WHERE user_id=$1::uuid`, userID, amount, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type postgresQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *PostgresStore) postgresFreeBillingStatus(ctx context.Context, userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	return s.postgresFreeBillingStatusTx(ctx, s.pool, userID, now, freeDailyCredits)
}

func (s *PostgresStore) postgresFreeBillingStatusTx(ctx context.Context, queryer postgresQueryer, userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var used int
	if err := queryer.QueryRow(ctx, `SELECT COALESCE(SUM(amount), 0) FROM credit_usages WHERE user_id=$1::uuid AND is_free=true AND created_at >= $2`, userID, dayStart).Scan(&used); err != nil {
		return domain.BillingStatus{}, err
	}
	return domain.BillingStatus{ProductCode: "free", ProductName: "免费学习者", CreditAllowance: freeDailyCredits, CreditBalance: max(0, freeDailyCredits-used)}, nil
}

func (s *PostgresStore) RecordMiniProgramUse(userID string, activity string, sourceID string, now time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO credit_usages (id, user_id, activity, source_id, amount, is_free, created_at) VALUES ($1, $2::uuid, $3, $4, 1, true, $5) ON CONFLICT(user_id, activity, source_id) DO NOTHING`, uuid.NewString(), userID, miniProgramUseActivity(activity), strings.TrimSpace(sourceID), now)
	if err != nil {
		return 0, err
	}
	return s.CountMiniProgramUses(userID, now)
}

func (s *PostgresStore) RefundMiniProgramUse(userID string, activity string, sourceID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `DELETE FROM credit_usages WHERE user_id = $1::uuid AND activity = $2 AND source_id = $3`, userID, miniProgramUseActivity(activity), strings.TrimSpace(sourceID))
	return err
}

func (s *PostgresStore) CountMiniProgramUses(userID string, now time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM credit_usages WHERE user_id = $1::uuid AND activity LIKE $2 AND created_at >= $3`, userID, miniProgramUseActivityPrefix+"%", dayStart).Scan(&count)
	return count, err
}

func scanPostgresPaymentOrder(row pgx.Row) (domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	var paidAt *time.Time
	if err := row.Scan(&order.ID, &order.UserID, &order.ProductCode, &order.AmountCents, &order.PaymentChannel, &order.Status, &order.ProviderTradeNo, &order.CreatedAt, &paidAt); err != nil {
		return domain.PaymentOrder{}, err
	}
	if paidAt != nil {
		order.PaidAt = *paidAt
	}
	return order, nil
}
