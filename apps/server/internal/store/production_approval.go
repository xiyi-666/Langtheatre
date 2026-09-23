package store

import (
	"encoding/json"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
)

func marshalProductionApproval(approval domain.ProductionApproval) ([]byte, error) {
	if approval.Status == "" {
		approval = contentquality.LegacyApproval()
	}
	return json.Marshal(approval)
}

func unmarshalProductionApproval(raw []byte) domain.ProductionApproval {
	approval := contentquality.LegacyApproval()
	if len(raw) == 0 || json.Unmarshal(raw, &approval) != nil || approval.Status == "" {
		return contentquality.LegacyApproval()
	}
	return approval
}
