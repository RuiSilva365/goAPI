package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Game represents a football match entity
type Game struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Team1    string    `json:"team1"`
	Team2    string    `json:"team2"`
	GameTime time.Time `json:"game_time"`
	League   string    `json:"league"`
	Status   string    `json:"status"`
	Season   int       `json:"season"`
}

// PredictionRequest represents a request to predict a game outcome
type PredictionRequest struct {
	Season   int    `json:"season"`
	League   string `json:"league"`
	Team1    string `json:"team1"`
	Team2    string `json:"team2"`
	GameDate string `json:"gameDate"`
}

// JobResponse represents the status of a prediction job
type JobResponse struct {
	Status  string                 `json:"status"`
	JobID   string                 `json:"job_id"`
	Message string                 `json:"message,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// DataResponse represents a response containing DataFrame data
type DataResponse struct {
	Status  string                   `json:"status"`
	Data    []map[string]interface{} `json:"data,omitempty"`
	Shape   []int                    `json:"shape,omitempty"`
	Columns []string                 `json:"columns,omitempty"`
	Message string                   `json:"message,omitempty"`
}

// LeaguesResponse represents a response containing available leagues
type LeaguesResponse struct {
	Status  string   `json:"status"`
	Leagues []string `json:"leagues,omitempty"`
}

// TeamsResponse represents a response containing teams for a league
type TeamsResponse struct {
	Status string   `json:"status"`
	League string   `json:"league,omitempty"`
	Teams  []string `json:"teams,omitempty"`
}

var games []Game
var pythonAPIURL = "http://localhost:5000/api" // URL of the Python Flask API

func main() {
	r := mux.NewRouter()

	// Game management endpoints
	r.HandleFunc("/games", getGames).Methods("GET")
	r.HandleFunc("/games/{id}", getGame).Methods("GET")
	r.HandleFunc("/games", createGame).Methods("POST")

	// ML Prediction endpoints
	r.HandleFunc("/predict", startPrediction).Methods("POST")
	r.HandleFunc("/jobs/{id}", getJobStatus).Methods("GET")
	r.HandleFunc("/data/team", getTeamData).Methods("GET")
	r.HandleFunc("/data/next-game", getNextGameData).Methods("GET")
	r.HandleFunc("/leagues", getLeagues).Methods("GET")
	r.HandleFunc("/teams", getTeams).Methods("GET")

	// Add middleware to handle CORS
	r.Use(corsMiddleware)
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Game management handlers
func getGames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

func getGame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)

	for _, game := range games {
		if game.ID == params["id"] {
			json.NewEncoder(w).Encode(game)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"message": "Game not found"})
}

func createGame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var game Game
	err := json.NewDecoder(r.Body).Decode(&game)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request"})
		return
	}

	games = append(games, game)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(game)
}

// ML Prediction handlers
func startPrediction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var predReq PredictionRequest
	err := json.NewDecoder(r.Body).Decode(&predReq)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request format"})
		return
	}

	// Forward request to Python API
	jsonData, err := json.Marshal(predReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to marshal request"})
		return
	}

	resp, err := http.Post(pythonAPIURL+"/predict", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	var jobResponse JobResponse
	err = json.Unmarshal(body, &jobResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to parse response"})
		return
	}

	// Return the job ID and status
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(jobResponse)
}

func getJobStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	jobID := params["id"]

	// Forward request to Python API
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", pythonAPIURL, jobID))
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	// Forward the response from Python API
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func getTeamData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	team := r.URL.Query().Get("team")
	if team == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Team parameter is required"})
		return
	}

	// Forward request to Python API
	resp, err := http.Get(fmt.Sprintf("%s/data/team?team=%s", pythonAPIURL, team))
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	// Forward the response from Python API
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func getNextGameData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Forward request to Python API
	resp, err := http.Get(pythonAPIURL + "/data/next-game")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	// Forward the response from Python API
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func getLeagues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Forward request to Python API
	resp, err := http.Get(pythonAPIURL + "/leagues")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	// Forward the response from Python API
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func getTeams(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	league := r.URL.Query().Get("league")
	if league == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "League parameter is required"})
		return
	}

	// Forward request to Python API
	resp, err := http.Get(fmt.Sprintf("%s/teams?league=%s", pythonAPIURL, league))
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"message": "Python API unavailable", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to read response"})
		return
	}

	// Forward the response from Python API
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
