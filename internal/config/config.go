package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	BoostyLink string `json:"boosty_link"`
	PriceText  string `json:"price_list"` // В JSON у тебя price_list
	AdminID    int64  `json:"admin_id"`
	Messages   struct {
		Greeting     string `json:"greeting"`
		Info         string `json:"info"`         // Инфо-блок
		Instructions string `json:"instructions"` // Инструкция по оплате
		GetEmail     string `json:"get_email"`
		GetPassword  string `json:"get_password"`
		Success      string `json:"success"`
	} `json:"messages"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(file, &cfg)
	return &cfg, err
}
