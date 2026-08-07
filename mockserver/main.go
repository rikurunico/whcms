// Command whcms-mock is a standalone mock of every external service the
// WHCMS backend integrates with: Duitku (payment gateway), WHM/cPanel,
// DirectAdmin, RDash (domain registrar) and an SMTP-replacement mail capture.
//
// All state lives in memory; POST /mock/reset wipes everything. See README.md.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg := configFromEnv()
	srv := NewServer(cfg)

	addr := ":" + cfg.Port
	log.Printf("whcms-mock listening on %s (duitku merchant=%s)", addr, cfg.DuitkuMerchantCode)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
