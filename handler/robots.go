package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	cacheClient "github.com/IliaW/robots-api/internal/cache"
	"github.com/IliaW/robots-api/internal/model"
	"github.com/IliaW/robots-api/internal/persistence"
	"github.com/IliaW/robots-api/util"
	"github.com/gin-gonic/gin"
	"github.com/jimsmart/grobotstxt"
)

type RobotsHandler struct {
	cache      cacheClient.CachedClient
	ruleRepo   persistence.RuleStorage
	httpClient *http.Client
}

func NewRobotsHandler(cache cacheClient.CachedClient, ruleRepo persistence.RuleStorage, httpClient *http.Client) *RobotsHandler {
	return &RobotsHandler{
		cache:      cache,
		ruleRepo:   ruleRepo,
		httpClient: httpClient,
	}
}

// GetAllowedScrape godoc
// @Summary Check if scraping is allowed for a specific user agent and URL
// @Description Check if the given user agent is allowed to scrape the specified URL based on the robots.txt rules
// @Tags Scraping
// @Produce plain
// @Param url query string true "URL to check"
// @Param user_agent query string true "User agent to check"
// @Success 200 {string} true "true or false depending on whether scraping is allowed"
// @Failure 400 {string} string "Bad request, missing 'url' or 'user_agent'"
// @Failure 500 {string} string "Internal server error"
// @Security ApiKeyAuth
// @Router /scrape-allowed [get]
func (h *RobotsHandler) GetAllowedScrape(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.String(http.StatusBadRequest, "error: 'url' query parameter is required")
		return
	}
	userAgent := c.Query("user_agent")
	if userAgent == "" {
		c.String(http.StatusBadRequest, "error: 'user_agent' query parameter is required")
		return
	}

	var robotsTxt string
	// check the custom rule for the given url in database
	rule, err := h.ruleRepo.GetByUrl(url)
	if err == nil && rule != nil && rule.RobotsTxt != "" {
		robotsTxt = rule.RobotsTxt
	} else {
		// upload the robots.txt file if custom rule is not found in database
		robotsTxt, err = h.getRobotsTxt(url)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: failed to load robots.txt. %s", err.Error()))
			return
		}
	}

	if ok := grobotstxt.AgentAllowed(robotsTxt, userAgent, url); ok {
		c.String(http.StatusOK, "true")
		return
	}

	c.String(http.StatusOK, "false")
}

// GetCustomRule godoc
// @Summary Get custom rule by ID or URL
// @Description Retrieve a custom rule based on the provided query parameter 'id' or 'url'
// @Tags Custom Rule
// @Produce json
// @Param id query string false "Custom rule ID"
// @Param url query string false "Custom rule URL"
// @Success 200 {object} model.Rule "Custom rule object"
// @Failure 400 {object} error "Bad request. Either 'id' or 'url' must be provided"
// @Failure 500 {object} error "Internal server error"
// @Security ApiKeyAuth
// @Router /custom-rule [get]
func (h *RobotsHandler) GetCustomRule(c *gin.Context) {
	id := c.Query("id")
	url := c.Query("url")
	if id == "" && url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'id' or 'url' query parameter is required"})
		return
	}

	if id != "" {
		rule, err := h.ruleRepo.GetById(id)
		if err != nil {
			c.JSON(http.StatusNotFound,
				gin.H{"error": fmt.Sprintf("failed to get rule by id. %s", err.Error())})
			return
		}
		c.JSON(http.StatusOK, rule)
		return
	}

	rule, err := h.ruleRepo.GetByUrl(url)
	if err != nil {
		c.JSON(http.StatusNotFound,
			gin.H{"error": fmt.Sprintf("failed to get rule by url. %s", err.Error())})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// CreateCustomRule godoc
// @Summary Create a custom rule
// @Description Create a new custom rule by providing a URL and the corresponding rule file
// @Tags Custom Rule
// @Accept plain
// @Produce json
// @Param url query string true "URL for the custom rule"
// @Param file body string true "Custom rule file content"
// @Success 200 {object} string "Custom rule created successfully"
// @Failure 400 {object} error "Bad request, missing 'url' or empty file"
// @Failure 500 {object} error "Internal server error"
// @Security ApiKeyAuth
// @Router /custom-rule [post]
func (h *RobotsHandler) CreateCustomRule(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'url' query parameter is required"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unable to read file. %s", err.Error())})
		return
	}
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "custom rules are not found or empty"})
		return
	}

	domain, err := util.GetDomain(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to parse url. %s", err.Error())})
		return
	}

	id, err := h.ruleRepo.Save(&model.Rule{
		Domain:    domain,
		RobotsTxt: string(body),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to save custom rule. %v", err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateCustomRule godoc
// @Summary Update a custom rule by ID
// @Description Update an existing custom rule based on the provided ID.
// @Tags Custom Rule
// @Accept plain
// @Produce json
// @Param id query string true "Custom rule ID"
// @Param url query string true "New URL for the custom rule"
// @Param file body string true "Updated custom rule file content"
// @Success 200 {object} model.Rule "Updated custom rule"
// @Failure 400 {object} error "Bad request, missing 'id' or invalid data to update"
// @Failure 404 {object} error "Rule not found"
// @Failure 500 {object} error "Internal server error"
// @Security ApiKeyAuth
// @Router /custom-rule [put]
func (h *RobotsHandler) UpdateCustomRule(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'id' query parameter is required"})
		return
	}

	rule, err := h.ruleRepo.GetById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	url := c.Query("url")
	domain, err := util.GetDomain(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to parse url. %s", err.Error())})
		return
	}
	rule.Domain = domain

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to read file"})
		return
	}
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "custom rules are not found or empty"})
		return
	}
	rule.RobotsTxt = string(body)

	result, err := h.ruleRepo.Update(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to update custom rule. %v", err.Error())})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteCustomRule godoc
// @Summary Delete a custom rule by ID
// @Description Delete an existing custom rule based on the provided ID.
// @Tags Custom Rule
// @Produce json
// @Param id query string true "Custom rule ID"
// @Success 200 {object} error "Rule deleted successfully"
// @Failure 400 {object} error "Bad request, missing 'id'"
// @Failure 500 {object} error "Internal server error"
// @Security ApiKeyAuth
// @Router /custom-rule [delete]
func (h *RobotsHandler) DeleteCustomRule(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'id' query parameter is required"})
		return
	}

	err := h.ruleRepo.Delete(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to delete custom rule. %v", err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("rule with id '%s' is deleted", id)})
}

func (h *RobotsHandler) getRobotsTxt(url string) (string, error) {
	// check if the robots.txt file is already saved in cache
	file, ok := h.cache.GetRobotsFile(url)
	if ok {
		return file, nil
	}
	// make get request to fetch the robots.txt file if it is not saved in cache
	resp, err := h.requestToRobotsTxt(url)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp) == 0 {
		return "", fmt.Errorf("empty response")
	}
	h.cache.SaveRobotsFile(url, resp)

	return string(resp), nil
}

func (h *RobotsHandler) requestToRobotsTxt(url string) ([]byte, error) {
	baseUrl, err := util.GetBaseUrl(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to parse url. %s", err.Error()))
	}
	req, err := http.NewRequest(http.MethodGet, baseUrl+"/robots.txt", nil)
	resp, err := h.httpClient.Do(req)
	defer func(Body io.ReadCloser) {
		err = resp.Body.Close()
		if err != nil {
			slog.Error("error closing response body", slog.String("err", err.Error()))
		}
	}(resp.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("error making http get request to %s/robots.txt", baseUrl),
			slog.String("err", err.Error()))
		return nil, err
	}

	if !isSuccess(resp.StatusCode) {
		slog.Warn("status code not successful", slog.String("code", resp.Status))
		return nil, err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("error reading response body", slog.String("err", err.Error()))
		return nil, err
	}
	return b, nil
}

func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
