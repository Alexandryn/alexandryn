package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// PairingVerifier executes the atomic verify-and-consume transaction
// (backend-network-api.md FR-2, ADR 0021).
type PairingVerifier struct {
	transactor  domain.Transactor
	sessionRepo *PairingSessionRepository
	deviceRepo  domain.PairedDeviceRepository
}

func NewPairingVerifier(
	transactor domain.Transactor,
	sessionRepo *PairingSessionRepository,
	deviceRepo domain.PairedDeviceRepository,
) *PairingVerifier {
	return &PairingVerifier{
		transactor:  transactor,
		sessionRepo: sessionRepo,
		deviceRepo:  deviceRepo,
	}
}

// VerifyAndConsume locks the pending pairing session row (SELECT ... FOR UPDATE),
// validates and consumes the session, saves the session state, and inserts
// the provisional paired device row inside a single atomic transaction (ADR 0021).
func (v *PairingVerifier) VerifyAndConsume(
	ctx context.Context,
	code domain.PairingCode,
	deviceID domain.DeviceID,
	label string,
	deviceClass domain.DeviceClass,
	now time.Time,
) (*domain.PairingSession, error) {
	trimmedLabel := strings.TrimSpace(label)
	if trimmedLabel == "" {
		trimmedLabel = string(deviceClass)
		if trimmedLabel == "" {
			trimmedLabel = "Device"
		}
	}
	if len([]rune(trimmedLabel)) > 100 {
		return nil, &domain.Error{Category: domain.InvalidInput, Message: "device label cannot exceed 100 characters"}
	}

	var verifiedSession *domain.PairingSession

	err := v.transactor.InTx(ctx, func(txCtx context.Context) error {
		codeIndex := v.sessionRepo.CodeIndex(code)

		// 1. SELECT ... FOR UPDATE with state='pending' and expires_at > now
		session, err := v.sessionRepo.FindPendingByCodeIndexForUpdate(txCtx, codeIndex, now)
		if err != nil {
			return err
		}

		// 2. session.Verify checks code match and advances pending -> verified
		if err := session.Verify(now, code, deviceID); err != nil {
			return err
		}

		// 3. session.Consume advances verified -> consumed
		if err := session.Consume(now); err != nil {
			return err
		}

		// 4. Save consumed session
		if err := v.sessionRepo.Save(txCtx, session); err != nil {
			return err
		}

		// 5. Insert provisional paired_device (owner_id IS NULL)
		if err := v.deviceRepo.InsertProvisional(
			txCtx,
			deviceID,
			label,
			deviceClass,
			domain.EnrolledViaPairingCode,
			session.ID(),
			now,
		); err != nil {
			return err
		}

		verifiedSession = session
		return nil
	})

	if err != nil {
		return nil, err
	}
	return verifiedSession, nil
}
