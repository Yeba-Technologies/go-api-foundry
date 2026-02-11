package waitlist

import (
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"gorm.io/gorm"
)

func NewWaitlistController(
	db *gorm.DB,
	logger *log.Logger,
) *router.RESTController {

	return router.NewVersionedRESTController(
		"WaitlistController",
		"v1",
		"/waitlist",
		func(rs *router.RouterService, c *router.RESTController) {
			repository := NewWaitlistRepository(db)
			service := NewWaitlistService(logger, repository)

			waitlistCreationLimiter := createWaitlistCreationRateLimiter(rs)

			rs.AddPostHandler(c, waitlistCreationLimiter, "", createWaitlistEntryHandler(service))
			rs.AddGetHandler(c, nil, "/:id", getWaitlistEntryHandler(service))
			rs.AddPutHandler(c, nil, "/:id", updateWaitlistEntryHandler(service))
			rs.AddGetHandler(c, nil, "", getAllWaitlistEntriesHandler(service))
			rs.AddDeleteHandler(c, nil, "/:id", deleteWaitlistEntryHandler(service))
		},
	)
}

func createWaitlistCreationRateLimiter(routerService *router.RouterService) ratelimit.RateLimiter {
	const waitlistCreationRequestsPerMinute = 30 // More permissive than monitoring (10/min)

	config := &ratelimit.RateLimitConfig{
		Requests: waitlistCreationRequestsPerMinute,
		Window:   time.Minute, // 1 minute window
		Redis:    nil,         // For now, use in-memory (could be enhanced to use Redis)
		Logger:   nil,         // Logger not needed for in-memory limiter
	}

	return ratelimit.NewRateLimiter(config)
}

func createWaitlistEntryHandler(service WaitlistService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		logger := router.GetLogger(ctx)

		var req CreateWaitlistEntryRequest

		if err := ctx.ShouldBindJSON(&req); err != nil {
			logger.Error("Failed to bind request", "error", err)

			validationErrors := apperrors.FormatValidationErrors(err, &req)
			if len(validationErrors) > 0 {
				return router.BadRequestResult("Invalid request payload", validationErrors)
			}

			return router.BadRequestResult("Invalid request body", nil)
		}

		response, err := service.CreateEntry(ctx.Request.Context(), &req)
		if err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.CreatedResult(response, "Waitlist entry")
	}
}

func getWaitlistEntryHandler(service WaitlistService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		id, errResult := router.ParseIDParam(ctx, "id")
		if errResult != nil {
			return errResult
		}

		response, err := service.FindEntryByID(ctx.Request.Context(), id)
		if err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.OKResult(response, "Waitlist entry retrieved successfully")
	}
}

func updateWaitlistEntryHandler(service WaitlistService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		logger := router.GetLogger(ctx)

		id, errResult := router.ParseIDParam(ctx, "id")
		if errResult != nil {
			return errResult
		}

		var req UpdateWaitlistEntryRequest

		if err := ctx.ShouldBindJSON(&req); err != nil {
			logger.Error("Failed to bind request", "error", err)

			validationErrors := apperrors.FormatValidationErrors(err, &req)
			if len(validationErrors) > 0 {
				return router.BadRequestResult("Invalid request payload", validationErrors)
			}

			return router.BadRequestResult("Invalid request body", nil)
		}

		if err := service.UpdateEntry(ctx.Request.Context(), id, &req); err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.OKResult(nil, "Waitlist entry updated successfully")
	}
}

func getAllWaitlistEntriesHandler(service WaitlistService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		response, err := service.GetAllEntries(ctx.Request.Context())
		if err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.OKResult(response, "Waitlist entries retrieved successfully")
	}
}

func deleteWaitlistEntryHandler(service WaitlistService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		id, errResult := router.ParseIDParam(ctx, "id")
		if errResult != nil {
			return errResult
		}

		if err := service.DeleteEntry(ctx.Request.Context(), id); err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.OKResult(nil, "Waitlist entry deleted successfully")
	}
}
