package store

import (
	"errors"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

func (s *MemoryStore) CreatePaymentOrder(order domain.PaymentOrder) (domain.PaymentOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.orders[order.ID]; exists {
		return domain.PaymentOrder{}, errors.New("payment order already exists")
	}
	s.orders[order.ID] = order
	return order, nil
}

func (s *MemoryStore) GetPaymentOrder(orderID string, userID string) (domain.PaymentOrder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order, ok := s.orders[orderID]
	if !ok || (userID != "" && order.UserID != userID) {
		return domain.PaymentOrder{}, errors.New("payment order not found")
	}
	return order, nil
}

func (s *MemoryStore) MarkPaymentOrderPaid(orderID string, providerTradeNo string, product domain.BillingProduct, paidAt time.Time) (domain.PaymentOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	if !ok {
		return domain.PaymentOrder{}, errors.New("payment order not found")
	}
	if order.Status == "PAID" {
		return order, nil
	}
	order.Status = "PAID"
	order.ProviderTradeNo = providerTradeNo
	order.PaidAt = paidAt
	s.orders[orderID] = order

	current, hasCurrent := s.billing[order.UserID]
	if hasCurrent && current.IsLifetime {
		current.CreditBalance += product.CreditAllowance
		s.billing[order.UserID] = current
		return order, nil
	}
	status := domain.BillingStatus{
		ProductCode:     product.Code,
		ProductName:     product.Name,
		IsLifetime:      product.Kind == "LIFETIME",
		AdsFree:         product.AdsFree,
		CreditBalance:   product.CreditAllowance,
		CreditAllowance: product.CreditAllowance,
		CreditResetAt:   paidAt.AddDate(0, 0, product.PeriodDays),
	}
	if !status.IsLifetime {
		status.ExpiresAt = paidAt.AddDate(0, 0, product.PeriodDays)
	}
	s.billing[order.UserID] = status
	return order, nil
}

func (s *MemoryStore) GetBillingStatus(userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.billingStatusLocked(userID, now, freeDailyCredits), nil
}

func (s *MemoryStore) ConsumeCredits(userID string, activity string, sourceID string, amount int, now time.Time, freeDailyCredits int) (domain.BillingStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryCreditKey(userID, activity, sourceID)
	status := s.billingStatusLocked(userID, now, freeDailyCredits)
	if _, duplicate := s.creditUses[key]; duplicate {
		return status, nil
	}
	if status.CreditBalance < amount {
		return status, errors.New("AI 点数不足，请等待每日点数重置")
	}
	use := memoryCreditUse{UserID: userID, Activity: strings.TrimSpace(activity), SourceID: strings.TrimSpace(sourceID), Amount: amount, CreatedAt: now, IsFree: status.ProductCode == "free"}
	s.creditUses[key] = use
	if use.IsFree {
		return s.billingStatusLocked(userID, now, freeDailyCredits), nil
	}
	paid := s.billing[userID]
	paid.CreditBalance -= amount
	s.billing[userID] = paid
	return paid, nil
}

func (s *MemoryStore) RefundCredits(userID string, activity string, sourceID string, amount int, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryCreditKey(userID, activity, sourceID)
	use, ok := s.creditUses[key]
	if !ok {
		return nil
	}
	delete(s.creditUses, key)
	if !use.IsFree {
		status, exists := s.billing[userID]
		if exists {
			status.CreditBalance += amount
			s.billing[userID] = status
		}
	}
	return nil
}

func (s *MemoryStore) billingStatusLocked(userID string, now time.Time, freeDailyCredits int) domain.BillingStatus {
	if status, ok := s.billing[userID]; ok {
		if status.IsLifetime {
			if !status.CreditResetAt.After(now) {
				status.CreditBalance = status.CreditAllowance
				status.CreditResetAt = now.AddDate(0, 0, 30)
				s.billing[userID] = status
			}
			return status
		}
		if status.ExpiresAt.After(now) {
			return status
		}
	}
	return s.freeBillingStatusLocked(userID, now, freeDailyCredits)
}

func (s *MemoryStore) freeBillingStatusLocked(userID string, now time.Time, freeDailyCredits int) domain.BillingStatus {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	used := 0
	for _, item := range s.creditUses {
		if item.UserID == userID && item.IsFree && !item.CreatedAt.Before(start) {
			used += item.Amount
		}
	}
	return domain.BillingStatus{ProductCode: "free", ProductName: "免费学习者", CreditAllowance: freeDailyCredits, CreditBalance: max(0, freeDailyCredits-used)}
}

func (s *MemoryStore) RecordMiniProgramUse(userID string, activity string, sourceID string, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryCreditKey(userID, miniProgramUseActivity(activity), sourceID)
	if _, exists := s.creditUses[key]; !exists {
		s.creditUses[key] = memoryCreditUse{UserID: userID, Activity: miniProgramUseActivity(activity), SourceID: strings.TrimSpace(sourceID), Amount: 1, CreatedAt: now, IsFree: true}
	}
	return s.countMiniProgramUsesLocked(userID, now), nil
}

func (s *MemoryStore) RefundMiniProgramUse(userID string, activity string, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.creditUses, memoryCreditKey(userID, miniProgramUseActivity(activity), sourceID))
	return nil
}

func (s *MemoryStore) CountMiniProgramUses(userID string, now time.Time) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.countMiniProgramUsesLocked(userID, now), nil
}

func (s *MemoryStore) countMiniProgramUsesLocked(userID string, now time.Time) int {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	count := 0
	for _, item := range s.creditUses {
		if item.UserID == userID && strings.HasPrefix(item.Activity, miniProgramUseActivityPrefix) && !item.CreatedAt.Before(start) {
			count++
		}
	}
	return count
}

func memoryCreditKey(userID string, activity string, sourceID string) string {
	return strings.Join([]string{userID, strings.TrimSpace(activity), strings.TrimSpace(sourceID)}, "\x00")
}
