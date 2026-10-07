package portfolio

import (
	_ "embed"
	"encoding/json"
)

//go:embed knowledge.json
var knowledge []byte

type Profile struct {
	Name         string   `json:"name"`
	Headline     string   `json:"headline"`
	Location     string   `json:"location"`
	Availability string   `json:"availability"`
	Bio          string   `json:"bio"`
	Focus        []string `json:"focus"`
	Services     []string `json:"services"`
	Experience   string   `json:"experience"`
}
type Project struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Year         int      `json:"year"`
	Technologies []string `json:"technologies"`
	URL          string   `json:"url"`
	Features     []string `json:"features"`
}
type Skill struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}
type Contact struct {
	Email       string  `json:"email"`
	WhatsApp    string  `json:"whatsapp"`
	WhatsAppURL string  `json:"whatsapp_url"`
	Portfolio   string  `json:"portfolio"`
	GitHub      *string `json:"github"`
}
type Service struct {
	Profile  Profile   `json:"profile"`
	Projects []Project `json:"projects"`
	Skills   []Skill   `json:"skills"`
	Contact  Contact   `json:"contact"`
}

func New() (*Service, error)       { var s Service; err := json.Unmarshal(knowledge, &s); return &s, err }
func (s *Service) Context() string { b, _ := json.Marshal(s); return string(b) }
