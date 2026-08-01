package response

import (
	stderrors "errors"
	"net/http"

	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id"`
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{Code: string(apperrors.CodeOK), Message: apperrors.Message(apperrors.CodeOK), Data: data, RequestID: requestID(c)})
}

func Failure(c *gin.Context, err error) {
	appError := apperrors.ErrInternal
	var typed *apperrors.AppError
	if stderrors.As(err, &typed) {
		appError = typed
	}
	status := appError.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	c.JSON(status, Envelope{Code: string(appError.Code), Message: apperrors.Message(appError.Code), Details: appError.Details, RequestID: requestID(c)})
}

func requestID(c *gin.Context) string {
	value, _ := c.Get("request_id")
	requestID, _ := value.(string)
	return requestID
}
