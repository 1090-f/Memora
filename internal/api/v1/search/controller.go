package search

import (
	"net/http"

	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	"github.com/gin-gonic/gin"
)

type Controller struct{ retrieval contracts.RetrievalService }

func NewController(retrieval contracts.RetrievalService) *Controller {
	return &Controller{retrieval: retrieval}
}

func (ctrl *Controller) Search(c *gin.Context) { ctrl.execute(c, false) }
func (ctrl *Controller) Test(c *gin.Context)   { ctrl.execute(c, true) }

func (ctrl *Controller) execute(c *gin.Context, debug bool) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	var input request.SearchRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	mode := contracts.RetrievalMode(input.Mode)
	if mode == "semantic" {
		mode = contracts.RetrievalVector
	}
	if mode == "" {
		mode = contracts.RetrievalHybrid
	}
	documentIDs := make([]contracts.ID, len(input.DocumentIDs))
	for i, id := range input.DocumentIDs {
		documentIDs[i] = contracts.ID(id)
	}
	result, err := ctrl.retrieval.Retrieve(c.Request.Context(), contracts.RetrievalRequest{
		UserID: contracts.ID(user.ID), KnowledgeBaseID: contracts.ID(c.Param("kb_id")),
		Query: input.Query, Mode: mode, DocumentIDs: documentIDs, TopK: input.TopK,
	})
	if err != nil {
		response.Failure(c, err)
		return
	}
	if !debug {
		response.Success(c, http.StatusOK, result)
		return
	}
	keyword, vector := make([]contracts.RetrievalItem, 0), make([]contracts.RetrievalItem, 0)
	for _, item := range result.Items {
		if item.KeywordRank != nil {
			keyword = append(keyword, item)
		}
		if item.VectorRank != nil {
			vector = append(vector, item)
		}
	}
	response.Success(c, http.StatusOK, gin.H{
		"query": result.Query, "keyword_results": keyword, "vector_results": vector,
		"rrf_results": result.Items, "reranked_results": result.Items, "final_results": result.Items,
		"knowledge_status": result.KnowledgeStatus,
		"timing":           gin.H{"total_ms": result.ElapsedMS},
	})
}
