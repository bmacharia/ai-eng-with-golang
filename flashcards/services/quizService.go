package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"flashcards/models"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	SYSTEM_PROMPT = `You are a quiz generator AI. Create educational quiz questions based on the provided study notes. Generate questions that test comprehension, application, and analysis of the material. Respond with valid JSON in this exact format:
{
  "question": "The question text here",
  "type": "multiple-choice",
  "options": ["A) Option 1", "B) Option 2", "C) Option 3", "D) Option 4"],
  "correctAnswer": "A",
  "explanation": "Explanation of why this is correct",
  "difficulty": "medium"
}

For essay questions, omit the options and correctAnswer fields. Valid difficulty levels are: easy, medium, hard. Valid types are: multiple-choice, essay, true-false.`

	USER_PROMPT_TEMPLATE = `Based on these study notes: 

%s

Generate a quiz question. Make it %s difficulty and format it as %s. The question should test understanding of the key concepts from the notes.`
)

type QuizService struct {
	noteService *NoteService
	llmClient   llms.Model
}

func NewQuizService(noteService *NoteService, apiKey string) (*QuizService, error) {
	log.Printf("[INFO] Initializing QuizService with OpenAI integration")
	
	if apiKey == "" {
		log.Printf("[ERROR] OpenAI API key is required but not provided")
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	llmClient, err := openai.New(
		openai.WithModel("gpt-4o-mini"),
		openai.WithToken(apiKey),
	)
	if err != nil {
		log.Printf("[ERROR] Failed to initialize OpenAI client: %v", err)
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	log.Printf("[INFO] QuizService initialized successfully with OpenAI GPT-4o-mini model")
	return &QuizService{
		noteService: noteService,
		llmClient:   llmClient,
	}, nil
}

func (s *QuizService) GenerateQuiz(conversation []models.Message, noteIds []int) (models.Message, error) {
	log.Printf("[INFO] Starting quiz generation with %d conversation messages and %d note IDs", len(conversation), len(noteIds))
	
	if len(conversation) == 0 {
		log.Printf("[ERROR] Quiz generation failed: conversation cannot be empty")
		return models.Message{}, fmt.Errorf("conversation cannot be empty")
	}

	lastMessage := conversation[len(conversation)-1]
	if lastMessage.Role != "user" {
		log.Printf("[ERROR] Quiz generation failed: last message must be from user, got role: %s", lastMessage.Role)
		return models.Message{}, fmt.Errorf("last message must be from user")
	}

	// Get notes content
	log.Printf("[INFO] Retrieving notes content for quiz generation")
	notesContent, err := s.getNotesContent(noteIds)
	if err != nil {
		log.Printf("[ERROR] Failed to retrieve notes content: %v", err)
		return models.Message{}, fmt.Errorf("failed to retrieve notes: %w", err)
	}

	// Generate quiz using LLM
	log.Printf("[INFO] Generating quiz using LLM with notes content length: %d characters", len(notesContent))
	difficulty := s.extractDifficulty(lastMessage.Content)
	questionType := s.extractQuestionType(lastMessage.Content)
	response, err := s.generateQuizWithLLM(notesContent, difficulty, questionType, noteIds)
	if err != nil {
		log.Printf("[ERROR] LLM quiz generation failed: %v", err)
		return models.Message{}, fmt.Errorf("failed to generate quiz with LLM: %w", err)
	}

	log.Printf("[INFO] Quiz generation completed successfully with question type: %s, difficulty: %s", questionType, difficulty)
	return response, nil
}

// Helper function to check if text contains any of the keywords
func (s *QuizService) containsKeywords(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// Get notes content for LLM input
func (s *QuizService) getNotesContent(noteIds []int) (string, error) {
	log.Printf("[INFO] Getting notes content - noteIds: %v", noteIds)
	
	var notes []*models.Note
	var err error

	if len(noteIds) == 0 {
		// Get all notes if no specific IDs provided
		log.Printf("[INFO] No specific note IDs provided, fetching all notes")
		notes, err = s.noteService.GetAllNotes()
		if err != nil {
			log.Printf("[ERROR] Failed to get all notes: %v", err)
			return "", err
		}
	} else {
		// Get specific notes by IDs
		log.Printf("[INFO] Fetching %d specific notes by ID", len(noteIds))
		for _, id := range noteIds {
			note, noteErr := s.noteService.GetNoteByID(id)
			if noteErr != nil {
				log.Printf("[ERROR] Failed to get note with ID %d: %v", id, noteErr)
				continue // Skip invalid note IDs
			}
			notes = append(notes, note)
		}
	}

	if len(notes) == 0 {
		log.Printf("[ERROR] No notes found for quiz generation")
		return "", fmt.Errorf("no notes found")
	}

	// Combine all notes content
	var contentBuilder strings.Builder
	for i, note := range notes {
		if i > 0 {
			contentBuilder.WriteString("\n\n---\n\n")
		}
		contentBuilder.WriteString(fmt.Sprintf("Note %d: %s", note.ID, note.Content))
	}

	content := contentBuilder.String()
	log.Printf("[INFO] Successfully combined %d notes into content with %d characters", len(notes), len(content))
	return content, nil
}

// Extract difficulty from user message
func (s *QuizService) extractDifficulty(message string) string {
	if s.containsKeywords(message, []string{"easy", "simple", "basic", "beginner"}) {
		log.Printf("[INFO] Extracted difficulty: easy from user message")
		return "easy"
	}
	if s.containsKeywords(message, []string{"hard", "difficult", "challenging", "advanced"}) {
		log.Printf("[INFO] Extracted difficulty: hard from user message")
		return "hard"
	}
	log.Printf("[INFO] Using default difficulty: medium")
	return "medium" // default
}

// Extract question type from user message
func (s *QuizService) extractQuestionType(message string) string {
	if s.containsKeywords(message, []string{"essay", "explain", "describe", "discuss"}) {
		log.Printf("[INFO] Extracted question type: essay from user message")
		return "essay"
	}
	if s.containsKeywords(message, []string{"true", "false", "yes", "no"}) {
		log.Printf("[INFO] Extracted question type: true-false from user message")
		return "true-false"
	}
	log.Printf("[INFO] Using default question type: multiple-choice")
	return "multiple-choice" // default
}

// Generate quiz using LLM
func (s *QuizService) generateQuizWithLLM(notesContent, difficulty, questionType string, noteIds []int) (models.Message, error) {
	log.Printf("[INFO] Starting LLM quiz generation - difficulty: %s, type: %s, noteIds: %v", difficulty, questionType, noteIds)
	startTime := time.Now()
	
	ctx := context.Background()

	// Prepare the prompt
	userPrompt := fmt.Sprintf(USER_PROMPT_TEMPLATE, notesContent, difficulty, questionType)
	log.Printf("[INFO] Prepared LLM prompt with %d characters", len(SYSTEM_PROMPT+"\n\n"+userPrompt))

	// Call LLM
	log.Printf("[INFO] Calling OpenAI LLM with temperature 0.9")
	completion, err := llms.GenerateFromSinglePrompt(
		ctx,
		s.llmClient,
		SYSTEM_PROMPT+"\n\n"+userPrompt,
		llms.WithTemperature(0.9),
	)
	if err != nil {
		log.Printf("[ERROR] LLM API call failed after %v: %v", time.Since(startTime), err)
		return models.Message{}, fmt.Errorf("LLM generation failed: %w", err)
	}

	log.Printf("[INFO] LLM API call completed successfully in %v, response length: %d characters", time.Since(startTime), len(completion))

	// Parse LLM response
	log.Printf("[INFO] Parsing LLM response to question data")
	questionData, err := s.parseLLMResponse(completion, noteIds)
	if err != nil {
		log.Printf("[ERROR] Failed to parse LLM response: %v", err)
		return models.Message{}, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Create message
	message := models.Message{
		Role:     "assistant",
		Content:  "Here's a quiz question based on your notes:",
		Question: &questionData,
	}

	log.Printf("[INFO] LLM quiz generation completed successfully - question ID: %s, type: %s", questionData.ID, questionData.Type)
	return message, nil
}

// Parse LLM JSON response into QuestionData
func (s *QuizService) parseLLMResponse(response string, noteIds []int) (models.QuestionData, error) {
	log.Printf("[INFO] Parsing LLM response with length: %d characters", len(response))
	
	var llmResponse struct {
		Question      string   `json:"question"`
		Type          string   `json:"type"`
		Options       []string `json:"options,omitempty"`
		CorrectAnswer string   `json:"correctAnswer,omitempty"`
		Explanation   string   `json:"explanation"`
		Difficulty    string   `json:"difficulty"`
	}

	// Clean the response - sometimes LLM adds extra text
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}") + 1
	if jsonStart == -1 || jsonEnd == 0 {
		log.Printf("[ERROR] No valid JSON found in LLM response")
		return models.QuestionData{}, fmt.Errorf("no valid JSON found in response")
	}
	
	jsonResponse := response[jsonStart:jsonEnd]
	log.Printf("[INFO] Extracted JSON from LLM response: %d characters", len(jsonResponse))

	if err := json.Unmarshal([]byte(jsonResponse), &llmResponse); err != nil {
		log.Printf("[ERROR] Failed to unmarshal JSON response: %v", err)
		return models.QuestionData{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if llmResponse.Question == "" {
		log.Printf("[ERROR] LLM response missing required question field")
		return models.QuestionData{}, fmt.Errorf("question field is required")
	}

	// Generate unique ID
	questionID := fmt.Sprintf("q_llm_%d", time.Now().Unix())

	questionData := models.QuestionData{
		ID:            questionID,
		Text:          llmResponse.Question,
		Type:          llmResponse.Type,
		Options:       llmResponse.Options,
		CorrectAnswer: llmResponse.CorrectAnswer,
		Explanation:   llmResponse.Explanation,
		Difficulty:    llmResponse.Difficulty,
		BasedOnNotes:  noteIds,
	}

	log.Printf("[INFO] Successfully parsed LLM response into question data - ID: %s, type: %s, difficulty: %s", 
		questionData.ID, questionData.Type, questionData.Difficulty)
	
	return questionData, nil
}
