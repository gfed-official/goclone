package models

type CustomCloneForm struct {
	Name       string   `json:"name"`
	Nat        bool     `json:"nat"`
	Vmstoclone []string `json:"vmstoclone"`
}
