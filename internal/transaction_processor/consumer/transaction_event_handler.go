package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/innoscripta-banking-ledger/internal/domain/shared"
	"github.com/innoscripta-banking-ledger/internal/platform/messaging/producers"
	"github.com/innoscripta-banking-ledger/internal/transaction_processor/service"
)

// TransactionEventHandler handles incoming transaction request messages from Kafka
type TransactionEventHandler struct {
	processingService service.ProcessingService
	producer          producers.DeadLetterPublisher
	logger            *slog.Logger
}

// NewTransactionEventHandler creates a new handler
func NewTransactionEventHandler(
	logger *slog.Logger,
	processingService service.ProcessingService,
	producer producers.DeadLetterPublisher, // Changed to specific interface
) *TransactionEventHandler {
	return &TransactionEventHandler{
		processingService: processingService,
		producer:          producer,
		logger:            logger,
	}
}

// HandleMessage processes Kafka messages
func (h *TransactionEventHandler) HandleMessage(ctx context.Context, key []byte, value []byte) error {
	var request shared.TransactionRequest
	if err := json.Unmarshal(value, &request); err != nil {
		unmarshalErrorMsg := "Failed to unmarshal transaction request from Kafka message"
		h.logger.Error(unmarshalErrorMsg,
			"error", err,
			"message_key", string(key),
		)

		// Send to DLQ if available
		if h.producer != nil {
			dlqReason := fmt.Sprintf("%s: %s", unmarshalErrorMsg, err.Error())
			if dlqErr := h.producer.PublishToDLQ(ctx, string(key), value, dlqReason); dlqErr != nil {
				h.logger.Error("Failed to publish message to DLQ after unmarshal error",
					"dlq_error", dlqErr,
					"original_error", err,
					"message_key", string(key),
				)
				// Return original error if DLQ fails
			} else {
				h.logger.Info("Successfully published unprocessable message to DLQ", "message_key", string(key), "reason", dlqReason)
				// Message handled, commit offset
				return nil
			}
		}
		// Allow Kafka retries
		return fmt.Errorf("failed to unmarshal message value: %w", err)
	}

	// Add correlation ID to logger
	logger := h.logger
	if request.CorrelationID != "" {
		logger = h.logger.With("correlation_id", request.CorrelationID)
	}

	logger.Info("Received transaction request for processing",
		"transaction_id", request.TransactionID.String(),
		"account_id", request.AccountID.String(),
		"type", request.Type,
		"amount", request.Amount,
	)

	if err := h.processingService.ProcessTransaction(ctx, &request); err != nil {
		logger.Error("Failed to process transaction",
			"transaction_id", request.TransactionID.String(),
			"account_id", request.AccountID.String(),
			"error", err,
		)
		return fmt.Errorf("processing transaction %s failed: %w", request.TransactionID.String(), err)
	}

	logger.Info("Successfully processed transaction", "transaction_id", request.TransactionID.String())
	return nil // Success, commit offset
}
