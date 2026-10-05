package server

import (
	"PgPoc/internal/model"
	"PgPoc/internal/persistence"
	"PgPoc/internal/worker"
	"compress/gzip"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, dispatcher *worker.Dispatcher) *gin.Engine {
	router := gin.Default()
	router.Use(gzipRequestMiddleWare)
	router.POST("/on_search", handleOnSearch(pool))
	router.POST("/search", handleSearch(dispatcher))
	return router
}

// handleSearch binds the query, enqueues it for async processing, and ACKs
// immediately. The rebuilt payload is delivered later via the recipient callback.
func handleSearch(dispatcher *worker.Dispatcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query model.SearchQuery
		if err := c.ShouldBindJSON(&query); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": "Unable to bind json",
			})
			return
		}
		if query.Recipient == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": "recipient is required",
			})
			return
		}
		dispatcher.Enqueue(query)
		c.JSON(http.StatusOK, gin.H{
			"message":  "Query queued",
			"query_id": query.QueryID,
		})
	}
}

func handleOnSearch(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload model.On_Search
		err := c.ShouldBindJSON(&payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"messsage": "Unable to bind json",
			})
		}

		if err := persistence.IngestOnSearch(c.Request.Context(), pool, payload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Request Ingested",
		})
	}
}

func gzipRequestMiddleWare(c *gin.Context) {
	if c.GetHeader("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "invalid gzip body",
			})
		}

		defer gz.Close()

		c.Request.Body = gz
	}
	c.Next()
}
