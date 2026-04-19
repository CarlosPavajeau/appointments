package state_machine

import (
	"context"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
)

func (s *service) Process(ctx context.Context, msg IncomingMessage) error {
	logger.Info("[scheduling] processing message",
		"tenant_id", msg.TenantID,
		"from", msg.From,
		"body", msg.Body,
		"interactive_id", msg.InteractiveID)

	customer, err := db.Query.FindCustomerByPhoneNumber(ctx, s.db.Primary(), db.FindCustomerByPhoneNumberParams{
		TenantID:    msg.TenantID,
		PhoneNumber: msg.From,
	})
	if err != nil {
		logger.Info("[scheduling] customer not found, creating new one",
			"tenant_id", msg.TenantID,
			"phone_number", msg.From)

		if err := db.Query.InsertCustomer(ctx, s.db.Primary(), db.InsertCustomerParams{
			TenantID:    msg.TenantID,
			PhoneNumber: msg.From,
		}); err != nil {
			logger.Error("[scheduling] failed to create customer",
				"tenant_id", msg.TenantID,
				"phone_number", msg.From,
				"err", err)
			return err
		}

		customer, err = db.Query.FindCustomerByPhoneNumber(ctx, s.db.Primary(), db.FindCustomerByPhoneNumberParams{
			TenantID:    msg.TenantID,
			PhoneNumber: msg.From,
		})

		if err != nil {
			logger.Error("[scheduling] failed to retrieve newly created customer",
				"tenant_id", msg.TenantID,
				"phone_number", msg.From,
				"err", err)
			return err
		}
	}

	if customer.IsBlocked {
		logger.Info("[scheduling] customer is blocked, ignoring message",
			"tenant_id", msg.TenantID,
			"phone_number", msg.From)
		return nil
	}

	session, err := db.Query.FindCustomerActiveConversationSession(ctx, s.db.Primary(), db.FindCustomerActiveConversationSessionParams{
		TenantID:   msg.TenantID,
		CustomerID: customer.ID,
	})

	if err != nil { // TODO: Assume session don't exist, improve this check
		return s.handleEntry(ctx, msg, customer)
	}

	switch SessionStep(session.Step) {
	case StepSelectService:
		return s.handleSelectService(ctx, msg, session)

	case StepSelectResource:
		return s.handleSelectResource(ctx, msg, session)

	case StepSelectDate:
		return s.handleSelectDate(ctx, msg, session, customer)

	case StepSelectTime:
		return s.handleSelectTime(ctx, msg, session, customer)

	case StepAwaitingName:
		return s.handleAwaitingName(ctx, msg, session, customer)

	case StepConfirm:
		return s.handleConfirm(ctx, msg, session, customer)

	default:
		logger.Warn("[scheduling] unknown step "+session.Step+" resetting to entry",
			"session_id", session.ID)

		if err := db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID); err != nil {
			logger.Warn("[scheduling] failed to delete session with unknown step, resetting to entry",
				"session_id", session.ID,
				"err", err)

		}

		return s.handleEntry(ctx, msg, customer)
	}
}
