package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/events"
)

type EventHandlers struct {
	broker *events.EventBroker
}

func NewEventHandlers(broker *events.EventBroker) *EventHandlers {
	return &EventHandlers{
		broker: broker,
	}
}

// StreamRequestEvents handles SSE connections for request events
func (h *EventHandlers) StreamRequestEvents(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse project ID from query params
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id query parameter required",
		})
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid project_id",
		})
		return
	}

	if ctxProjectID, ok := contexts.GetProjectID(ctx); ok && ctxProjectID != projectID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "project access denied",
		})
		return
	}

	// Subscribe to request events for this project
	subscriber := h.broker.Subscribe(ctx, events.TopicRequests, &projectID)
	defer h.broker.Unsubscribe(subscriber.ID)

	// Set SSE headers
	c.Header("Content-Type", sse.ContentType)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Send initial connection confirmation
	c.SSEvent("connected", gin.H{
		"subscriber_id": subscriber.ID,
		"timestamp":     time.Now().Unix(),
	})
	c.Writer.Flush()
	// Heartbeat ticker to keep connection alive
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return

		case <-heartbeatTicker.C:
			// Send heartbeat to keep connection alive
			c.SSEvent("heartbeat", gin.H{
				"timestamp": time.Now().Unix(),
			})
			c.Writer.Flush()

		case event, ok := <-subscriber.Events:
			if !ok {
				// Channel closed (broker shutdown)
				return
			}

			// Send event to client
			c.SSEvent(string(event.Type), event.Payload)
			c.Writer.Flush()

		}
	}
}
