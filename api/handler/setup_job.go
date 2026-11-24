package handler

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/service"
)

type setupJob struct {
	user   uint
	broker string
	result gin.H
}

func (c *GmailController) startSetupJob(ctx *gin.Context, uid uint, broker string) {
	brokers, err := c.service.Brokers.GetAllBrokers(uid)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	for _, b := range brokers.Data {
		if strings.EqualFold(b.Name, broker) {
			id := c.enqueueSetup(uid, b.Name, ctx.Query("gemini") == "1")
			ctx.JSON(http.StatusAccepted, gin.H{"job_id": id})
			return
		}
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"error": "Broker belum terhubung ke akun ini."})
}

func (c *GmailController) enqueueSetup(uid uint, broker string, gemini bool) string {
	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	for id, job := range c.jobs {
		if job.user == uid && job.broker == broker && job.result == nil {
			return id
		}
	}
	if c.jobs == nil {
		c.jobs = make(map[string]*setupJob)
	}
	id := rand.Text()
	c.jobs[id] = &setupJob{user: uid, broker: broker}
	go c.executeSetup(id, uid, broker, gemini)
	return id
}

func (c *GmailController) executeSetup(id string, uid uint, broker string, gemini bool) {
	var result gin.H
	defer func() {
		if recover() != nil {
			log.Printf("Setup Gmail %s berhenti karena kesalahan internal", id)
			result = gin.H{"error": "Impor broker terhenti. Silakan ulangi sinkronisasi."}
		}
		c.jobsMu.Lock()
		c.jobs[id].result = result
		c.jobsMu.Unlock()
		time.AfterFunc(15*time.Minute, func() {
			c.jobsMu.Lock()
			defer c.jobsMu.Unlock()
			delete(c.jobs, id)
		})
	}()
	result, _ = c.runSetup(service.WithGeminiSync(context.Background(), gemini), uid, broker, true)
}

func (c *GmailController) SetupJobStatus(ctx *gin.Context) {
	uid, ok := ctx.Get("user_id")
	user, valid := uid.(float64)
	if !ok || !valid {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	ctx.Header("Cache-Control", "no-store")
	c.jobsMu.Lock()
	job := c.jobs[ctx.Param("id")]
	if job == nil || job.user != uint(user) {
		c.jobsMu.Unlock()
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status impor tidak tersedia. Silakan ulangi sinkronisasi."})
		return
	}
	result := job.result
	c.jobsMu.Unlock()
	if result == nil {
		ctx.JSON(http.StatusAccepted, gin.H{"running": true})
		return
	}
	if result["error"] != nil {
		ctx.JSON(http.StatusInternalServerError, result)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"running": false, "result": result})
}
