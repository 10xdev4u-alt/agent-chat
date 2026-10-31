package web

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

//go:embed templates/*.html
var templatesFS embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(templatesFS, "templates/*.html"))
}

// Handler returns an http.Handler for the web UI.
func Handler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleDashboard(w, r, db)
	})
	mux.HandleFunc("/rooms", func(w http.ResponseWriter, r *http.Request) {
		handleRooms(w, r, db)
	})
	mux.HandleFunc("/rooms/", func(w http.ResponseWriter, r *http.Request) {
		handleRoom(w, r, db)
	})
	mux.HandleFunc("/approvals", func(w http.ResponseWriter, r *http.Request) {
		handleApprovals(w, r, db)
	})

	return mux
}

func handleDashboard(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.Query(`SELECT id, name, harness, status, doing, repo, branch FROM agents ORDER BY name`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type AgentInfo struct {
		ID      string
		Name    string
		Harness string
		Status  string
		Doing   string
		Repo    string
		Branch  string
	}
	var agents []AgentInfo
	for rows.Next() {
		var a AgentInfo
		var doing, repo, branch sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &a.Harness, &a.Status, &doing, &repo, &branch); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.Doing = doing.String
		a.Repo = repo.String
		a.Branch = branch.String
		agents = append(agents, a)
	}

	rooms, _ := rooms.ListRooms(db)

	var pendingCount int
	db.QueryRow(`SELECT COUNT(*) FROM approvals WHERE status = 'pending'`).Scan(&pendingCount)

	data := map[string]any{
		"Agents":       agents,
		"Rooms":        rooms,
		"PendingCount": pendingCount,
	}
	templates.ExecuteTemplate(w, "dashboard.html", data)
}

func handleRooms(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rooms, err := rooms.ListRooms(db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	templates.ExecuteTemplate(w, "rooms.html", map[string]any{"Rooms": rooms})
}

func handleRoom(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	roomID := r.URL.Path[len("/rooms/"):]
	if roomID == "" {
		http.Redirect(w, r, "/rooms", 302)
		return
	}
	room, err := rooms.GetRoom(db, roomID)
	if err != nil {
		http.Error(w, "room not found", 404)
		return
	}
	members, _ := rooms.ListMembers(db, roomID)
	msgs, _ := rooms.ReadMessages(db, roomID, 0, 100)

	templates.ExecuteTemplate(w, "room.html", map[string]any{
		"Room":     room,
		"Members":  members,
		"Messages": msgs,
	})
}

func handleApprovals(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.Query(`
		SELECT a.id, a.action, a.description, a.risk_level, a.status, a.created_at,
		       ag.name as requester_name, r.name as room_name
		FROM approvals a
		JOIN agents ag ON ag.id = a.requester_id
		JOIN rooms r ON r.id = a.room_id
		WHERE a.status = 'pending'
		ORDER BY a.created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type ApprovalInfo struct {
		ID            string
		Action        string
		Description   string
		RiskLevel     string
		Status        string
		CreatedAt     string
		RequesterName string
		RoomName      string
	}
	var approvals []ApprovalInfo
	for rows.Next() {
		var a ApprovalInfo
		var desc sql.NullString
		if err := rows.Scan(&a.ID, &a.Action, &desc, &a.RiskLevel, &a.Status, &a.CreatedAt, &a.RequesterName, &a.RoomName); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.Description = desc.String
		approvals = append(approvals, a)
	}

	templates.ExecuteTemplate(w, "approvals.html", map[string]any{"Approvals": approvals})
}

var _ = fmt.Sprintf
