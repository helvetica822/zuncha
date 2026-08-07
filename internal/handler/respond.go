package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

const (
	contentTypeJSON = "application/json"
	contentTypeWAV  = "audio/wav"
)

// respondJSON は status と data を JSON で返す。
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("レスポンスのエンコードに失敗: %v", err)
	}
}

// respondError は status とエラーメッセージを JSON で返す。
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
