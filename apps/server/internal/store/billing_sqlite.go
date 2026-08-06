package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

func (s *SQLiteStore) CreatePaymentOrder(order domain.PaymentOrder) (domain.PaymentOrder, error) {
	_, err := s.db.Exec(`INSERT INTO payment_orders (id, user_id, product_code, amount_cents, payment_channel, status, provider_trade_no, created_at)
		VALUES (?, ?, ?, ?, ?, ?, '', ?)`, order.ID, order.UserID, order.ProductCode, order.AmountCents, order.PaymentChannel, order.Status, order.CreatedAt.Format(sqliteTimeLayout))
	return order, err
}

func (s *SQLiteStore) GetPaymentOrder(orderID string, userID string) (domain.PaymentOrder, error) {
	query := `SELECT id, user_id, product_code, amount_cents, payment_channel, status, provider_trade_no, created_at, paid_at FROM payment_orders WHERE id = ?`
	args := []any{orderID}
	if strings.TrimSpace(userID) != "" {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	return scanSQLitePaymentOrder(s.db.QueryRow(query, args...))
}

func (s *SQLiteStore) MarkPaymentOrderPaid(orderID string, providerTradeNo string, product domain.BillingProduct, paidAt time.Time) (domain.PaymentOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	defer tx.Rollback()
	order, err := scanSQLitePaymentOrder(tx.QueryRow(`SELECT id, user_id, product_code, amount_cents, payment_channel, status, provider_trade_no, created_at, paid_at FROM payment_orders WHERE id = ?`, orderID))
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	if order.Status == "PAID" {
		return order, tx.Commit()
	}
	if _, err = tx.Exec(`UPDATE payment_orders SET status = 'PAID', provider_trade_no = ?, paid_at = ? WHERE id = ? AND status = 'PENDING'`, providerTradeNo, paidAt.Format(sqliteTimeLayout), orderID); err != nil {
		return domain.PaymentOrder{}, err
	}
	var isLifetime int
	currentErr := tx.QueryRow(`SELECT is_lifetime FROM billing_entitlements WHERE user_id = ?`, order.UserID).Scan(&isLifetime)
	if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
		return domain.PaymentOrder{}, currentErr
	}
	if isLifetime == 1 {
		_, err = tx.Exec(`UPDATE billing_entitlements SET credit_balance = credit_balance + ?, updated_at = ? WHERE user_id = ?`, product.CreditAllowance, paidAt.Format(sqliteTimeLayout), order.UserID)
	} else {
		isLifetime = 0
		if product.Kind == "LIFETIME" {
			isLifetime = 1
		}
		var expires any
		if isLifetime == 0 {
			expires = paidAt.AddDate(0, 0, product.PeriodDays).Format(sqliteTimeLayout)
		}
		_, err = tx.Exec(`INSERT INTO billing_entitlements (user_id, product_code, product_name, is_lifetime, ads_free, credit_balance, credit_allowance, credit_reset_at, expires_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET product_code=excluded.product_code, product_name=excluded.product_name, is_lifetime=excluded.is_lifetime, ads_free=excluded.ads_free, credit_balance=excluded.credit_balance, credit_allowance=excluded.credit_allowance, credit_reset_at=excluded.credit_reset_at, expires_at=excluded.expires_at, updated_at=excluded.updated_at`,
			order.UserID, product.Code, product.Name, isLifetime, boolInt(product.AdsFree), product.CreditAllowance, product.CreditAllowance, paidAt.AddDate(0, 0, product.PeriodDays).Format(sqliteTimeLayout), expires, paidAt.Format(sqliteTimeLayout))
	}
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.PaymentOrder{}, err
	}
	order.Status = "PAID"
	order.ProviderTradeNo = providerTradeNo
	order.PaidAt = paidAt
	return order, nil
}

func (s *SQLiteStore) GetBillingStatus(userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	var productCode, productName, creditResetRaw string
	var isLifetime, adsFree, balance, allowance int
	var expiresRaw sql.NullString
	err := s.db.QueryRow(`SELECT product_code, product_name, is_lifetime, ads_free, credit_balance, credit_allowance, COALESCE(credit_reset_at, ''), expires_at FROM billing_entitlements WHERE user_id = ?`, userID).Scan(&productCode, &productName, &isLifetime, &adsFree, &balance, &allowance, &creditResetRaw, &expiresRaw)
	if err == nil {
		resetAt, parseErr := parseSQLiteBillingTime(creditResetRaw)
		if parseErr != nil {
			return domain.BillingStatus{}, parseErr
		}
		var expiresAt time.Time
		if expiresRaw.Valid && expiresRaw.String != "" {
			expiresAt, parseErr = parseSQLiteBillingTime(expiresRaw.String)
			if parseErr != nil {
				return domain.BillingStatus{}, parseErr
			}
		}
		if isLifetime == 1 && !resetAt.After(now) {
			balance = allowance
			resetAt = now.AddDate(0, 0, 30)
			if _, err = s.db.Exec(`UPDATE billing_entitlements SET credit_balance = ?, credit_reset_at = ?, updated_at = ? WHERE user_id = ?`, balance, resetAt.Format(sqliteTimeLayout), now.Format(sqliteTimeLayout), userID); err != nil {
				return domain.BillingStatus{}, err
			}
		}
		if isLifetime == 1 || expiresAt.After(now) {
			return domain.BillingStatus{ProductCode: productCode, ProductName: productName, IsLifetime: isLifetime == 1, AdsFree: adsFree == 1, CreditBalance: balance, CreditAllowance: allowance, CreditResetAt: resetAt, ExpiresAt: expiresAt}, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.BillingStatus{}, err
	}
	return s.sqliteFreeBillingStatus(userID, now, freeDailyCredits)
}

func (s *SQLiteStore) ConsumeCredits(userID string, activity string, sourceID string, amount int, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	status, err := s.GetBillingStatus(userID, now, freeDailyCredits)
	if err != nil {
		return domain.BillingStatus{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.BillingStatus{}, err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRow(`SELECT 1 FROM credit_usages WHERE user_id = ? AND activity = ? AND source_id = ?`, userID, activity, sourceID).Scan(&exists)
	if err == nil {
		if err = tx.Commit(); err != nil {
			return domain.BillingStatus{}, err
		}
		return s.GetBillingStatus(userID, now, freeDailyCredits)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.BillingStatus{}, err
	}
	isFree := status.ProductCode == "free"
	if isFree {
		freeStatus, statusErr := s.sqliteFreeBillingStatusTx(tx, userID, now, freeDailyCredits)
		if statusErr != nil {
			return domain.BillingStatus{}, statusErr
		}
		if freeStatus.CreditBalance < amount {
			return freeStatus, errors.New("AI 点数不足，请等待每日点数重置")
		}
	} else {
		result, updateErr := tx.Exec(`UPDATE billing_entitlements SET credit_balance = credit_balance - ?, updated_at = ? WHERE user_id = ? AND credit_balance >= ?`, amount, now.Format(sqliteTimeLayout), userID, amount)
		if updateErr != nil {
			return domain.BillingStatus{}, updateErr
		}
		if changed, _ := result.RowsAffected(); changed == 0 {
			return status, errors.New("AI 点数不足，请等待每日点数重置")
		}
	}
	_, err = tx.Exec(`INSERT INTO credit_usages (id, user_id, activity, source_id, amount, is_free, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), userID, activity, sourceID, amount, boolInt(isFree), now.Format(sqliteTimeLayout))
	if err != nil {
		return domain.BillingStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.BillingStatus{}, err
	}
	return s.GetBillingStatus(userID, now, freeDailyCredits)
}

func (s *SQLiteStore) RefundCredits(userID string, activity string, sourceID string, amount int, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var isFree int
	err = tx.QueryRow(`SELECT is_free FROM credit_usages WHERE user_id = ? AND activity = ? AND source_id = ?`, userID, activity, sourceID).Scan(&isFree)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM credit_usages WHERE user_id = ? AND activity = ? AND source_id = ?`, userID, activity, sourceID); err != nil {
		return err
	}
	if isFree == 0 {
		_, err = tx.Exec(`UPDATE billing_entitlements SET credit_balance = credit_balance + ?, updated_at = ? WHERE user_id = ?`, amount, now.Format(sqliteTimeLayout), userID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) sqliteFreeBillingStatus(userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	return s.sqliteFreeBillingStatusTx(s.db, userID, now, freeDailyCredits)
}

type sqliteQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func (s *SQLiteStore) sqliteFreeBillingStatusTx(queryer sqliteQueryer, userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var used int
	if err := queryer.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM credit_usages WHERE user_id = ? AND is_free = 1 AND created_at >= ?`, userID, dayStart.Format(sqliteTimeLayout)).Scan(&used); err != nil {
		return domain.BillingStatus{}, err
	}
	return domain.BillingStatus{ProductCode: "free", ProductName: "免费学习者", CreditAllowance: freeDailyCredits, CreditBalance: max(0, freeDailyCredits-used)}, nil
}

func (s *SQLiteStore) RecordMiniProgramUse(userID string, activity string, sourceID string, now time.Time) (int, error) {
	_, err := s.db.Exec(`INSERT INTO credit_usages (id, user_id, activity, source_id, amount, is_free, created_at) VALUES (?, ?, ?, ?, 1, 1, ?) ON CONFLICT(user_id, activity, source_id) DO NOTHING`, uuid.NewString(), userID, miniProgramUseActivity(activity), strings.TrimSpace(sourceID), now.Format(sqliteTimeLayout))
	if err != nil {
		return 0, err
	}
	return s.CountMiniProgramUses(userID, now)
}

func (s *SQLiteStore) RefundMiniProgramUse(userID string, activity string, sourceID string) error {
	_, err := s.db.Exec(`DELETE FROM credit_usages WHERE user_id = ? AND activity = ? AND source_id = ?`, userID, miniProgramUseActivity(activity), strings.TrimSpace(sourceID))
	return err
}

func (s *SQLiteStore) CountMiniProgramUses(userID string, now time.Time) (int, error) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM credit_usages WHERE user_id = ? AND activity LIKE ? AND created_at >= ?`, userID, miniProgramUseActivityPrefix+"%", dayStart.Format(sqliteTimeLayout)).Scan(&count)
	return count, err
}

func scanSQLitePaymentOrder(row *sql.Row) (domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	var createdRaw string
	var paidRaw sql.NullString
	if err := row.Scan(&order.ID, &order.UserID, &order.ProductCode, &order.AmountCents, &order.PaymentChannel, &order.Status, &order.ProviderTradeNo, &createdRaw, &paidRaw); err != nil {
		return domain.PaymentOrder{}, err
	}
	created, err := parseSQLiteBillingTime(createdRaw)
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	order.CreatedAt = created
	if paidRaw.Valid && paidRaw.String != "" {
		paid, parseErr := parseSQLiteBillingTime(paidRaw.String)
		if parseErr != nil {
			return domain.PaymentOrder{}, parseErr
		}
		order.PaidAt = paid
	}
	return order, nil
}

func parseSQLiteBillingTime(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return time.Parse(sqliteTimeLayout, raw)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
