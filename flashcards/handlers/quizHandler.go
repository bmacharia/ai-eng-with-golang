package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"flashcards/models"
	"flashcards/services"

	"github.com/gorilla/mux"
)

// Request and Response structs local to the handler
type QuizRequest struct {
	NoteIds      []int            `json:"noteIds"`
	Conversation []models.Message `json:"conversation"`
	Options      struct {
		Difficulty   string `json:"difficulty,omitempty"`
		QuestionType string `json:"questionType,omitempty"`
	} `json:"options"`
}

type QuizResponse struct {
	Success  bool             `json:"success"`
	Data     QuizResponseData `json:"data"`
	Metadata QuizMetadata     `json:"metadata"`
}

type QuizResponseData struct {
	Conversation []models.Message `json:"conversation"`
}

type QuizMetadata struct {
	GeneratedAt      string `json:"generatedAt"`
	TokensUsed       int    `json:"tokensUsed"`
	ProcessingTimeMs int    `json:"processingTimeMs"`
}

type QuizHandler struct {
	service *services.QuizService
}

func NewQuizHandler(service *services.QuizService) *QuizHandler {
	return &QuizHandler{service: service}
}

func (h *QuizHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/notes/generate-quiz", h.GenerateQuiz).Methods("POST")
}

func (h *QuizHandler) GenerateQuiz(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	var req QuizRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validateQuizRequest(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Generate new assistant message
	newMessage, err := h.service.GenerateQuiz(req.Conversation, req.NoteIds)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate quiz: "+err.Error())
		return
	}

	// Append new assistant message to conversation
	updatedConversation := append(req.Conversation, newMessage)

	// Build response
	response := QuizResponse{
		Success: true,
		Data: QuizResponseData{
			Conversation: updatedConversation,
		},
		Metadata: QuizMetadata{
			GeneratedAt:      time.Now().Format(time.RFC3339),
			TokensUsed:       150, // Hardcoded for now
			ProcessingTimeMs: int(time.Since(startTime).Milliseconds()),
		},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *QuizHandler) validateQuizRequest(req *QuizRequest) error {
	if len(req.Conversation) == 0 {
		return fmt.Errorf("conversation cannot be empty")
	}

	lastMessage := req.Conversation[len(req.Conversation)-1]
	if lastMessage.Role != "user" {
		return fmt.Errorf("last message must be from user")
	}

	if lastMessage.Content == "" {
		return fmt.Errorf("user message cannot be empty")
	}

	return nil
}

func (h *QuizHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (h *QuizHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
