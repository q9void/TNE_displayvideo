package endpoints

import (
	"net/http"
)

// HandleSellersJSON serves the IAB sellers.json file for supply chain transparency.
// This file declares all authorized sellers in the TNE ad exchange, enabling buyers
// to verify the legitimacy of inventory sources via the IAB SupplyChain object.
//
// Standards:
// - IAB Sellers.json v1.0
// - https://iabtechlab.com/sellers-json/
func HandleSellersJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hour cache

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, "assets/sellers.json")
}
