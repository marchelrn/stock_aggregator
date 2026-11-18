package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"

	"github.com/marchelrn/stock_api/models"
)

//go:embed templates/trades_v2.json
var metadataTradeTemplates []byte

type PromoteMetadataTradeTemplates struct{}

func (*PromoteMetadataTradeTemplates) Name() string   { return "promote_metadata_trade_templates" }
func (*PromoteMetadataTradeTemplates) SkipProd() bool { return false }

func legacyTradeConfig(key string) (string, error) {
	if key == "trimegah" {
		return trimegahV2Config, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(stockbitV1Config, &cfg); err != nil {
		return "", err
	}
	cfg["schema_version"] = 2
	cfg["document"] = "pdf"
	cfg["kind"] = "trade"
	cfg["parse_mode"] = "legacy"
	cfg["legacy_parser"] = "stockbit"
	data, err := json.Marshal(cfg)
	return string(data), err
}

func (*PromoteMetadataTradeTemplates) Up(tx *sql.Tx) error {
	var seeds []models.PDFParsingTemplate
	if err := json.Unmarshal(metadataTradeTemplates, &seeds); err != nil {
		return err
	}
	for _, seed := range seeds {
		var id int64
		err := tx.QueryRow(`INSERT INTO pdf_parsing_templates
			(key,broker_name,version,active,config,email_sender,subject_contains,filename_suffix)
			VALUES ($1,$2,$3,FALSE,$4::jsonb,$5,$6,$7)
			ON CONFLICT (key,version) DO NOTHING RETURNING id`,
			seed.Key, seed.BrokerName, seed.Version, string(seed.Config), seed.EmailSender, seed.SubjectContains, seed.FilenameSuffix).Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if seed.Key == "stockbit.eipo" {
			// Activate only if the known regular template has been promoted;
			// otherwise a customized broad Stockbit route could overlap it.
			_, err = tx.Exec(`UPDATE pdf_parsing_templates SET active=TRUE WHERE id=$1
				AND EXISTS (SELECT 1 FROM pdf_parsing_templates WHERE key='stockbit'
				AND version=2 AND active=TRUE AND config=$2::jsonb)`, id, string(seeds[0].Config))
			if err != nil {
				return err
			}
			continue
		}
		old, err := legacyTradeConfig(seed.Key)
		if err != nil {
			return err
		}
		result, err := tx.Exec(`UPDATE pdf_parsing_templates SET active=FALSE,updated_at=NOW()
			WHERE key=$1 AND version=1 AND active=TRUE AND config=$2::jsonb
			AND email_sender=$3 AND subject_contains=$4 AND filename_suffix=''`,
			seed.Key, old, seed.EmailSender, seed.SubjectContains)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 1 {
			if _, err := tx.Exec(`UPDATE pdf_parsing_templates SET active=TRUE,updated_at=NOW() WHERE id=$1`, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (*PromoteMetadataTradeTemplates) Down(tx *sql.Tx) error {
	var seeds []models.PDFParsingTemplate
	if err := json.Unmarshal(metadataTradeTemplates, &seeds); err != nil {
		return err
	}
	for _, seed := range seeds {
		var active bool
		err := tx.QueryRow(`DELETE FROM pdf_parsing_templates WHERE key=$1 AND version=$2
			AND config=$3::jsonb AND email_sender=$4 AND subject_contains=$5
			AND filename_suffix=$6 RETURNING active`,
			seed.Key, seed.Version, string(seed.Config), seed.EmailSender, seed.SubjectContains, seed.FilenameSuffix).Scan(&active)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if !active || seed.Key == "stockbit.eipo" {
			continue
		}
		old, err := legacyTradeConfig(seed.Key)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE pdf_parsing_templates SET active=TRUE,updated_at=NOW()
			WHERE key=$1 AND version=1 AND config=$2::jsonb`, seed.Key, old); err != nil {
			return err
		}
	}
	return nil
}
