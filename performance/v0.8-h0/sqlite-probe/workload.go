//go:build modernc

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

const notes = 1000

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) != 3 || (os.Args[1] != "run" && os.Args[1] != "roundtrip") {
		panic("usage: modernc-workload run|roundtrip DB")
	}
	db, err := sql.Open("sqlite", os.Args[2])
	must(err)
	defer db.Close()
	start := time.Now()
	if os.Args[1] == "roundtrip" {
		var count, fts int
		var source, journal, integrity string
		must(db.QueryRow(`SELECT count(*) FROM notes`).Scan(&count))
		must(db.QueryRow(`SELECT count(*) FROM notes_fts WHERE notes_fts MATCH 'searchable'`).Scan(&fts))
		must(db.QueryRow(`SELECT json_extract(metadata,'$.source') FROM notes WHERE id=1`).Scan(&source))
		must(db.QueryRow(`PRAGMA journal_mode`).Scan(&journal))
		must(db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity))
		if count != 1000 || fts != 1000 || source != "synthetic" || journal != "wal" || integrity != "ok" {
			panic("pre-roundtrip assertions failed")
		}
		_, err = db.Exec(`INSERT OR REPLACE INTO notes(id,title,body,metadata,updated_at) VALUES(1001,'Modernc round-trip','written by the modernc candidate','{"source":"modernc"}',1001); INSERT INTO notes_fts(notes_fts) VALUES('rebuild')`)
		must(err)
		var row int
		must(db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity))
		must(db.QueryRow(`SELECT count(*) FROM notes WHERE id=1001 AND title='Modernc round-trip' AND body='written by the modernc candidate' AND json_extract(metadata,'$.source')='modernc'`).Scan(&row))
		if integrity != "ok" || row != 1 {
			panic("post-roundtrip assertions failed")
		}
		_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
		must(err)
		fmt.Printf("{\"candidate\":\"modernc\",\"roundtrip_notes\":%d,\"pre_fts_matches\":%d,\"integrity\":\"%s\",\"journal_mode\":\"%s\",\"modernc_row\":true,\"elapsed_ms\":%d}\n", count+1, fts, integrity, journal, time.Since(start).Milliseconds())
		return
	}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS notes(id INTEGER PRIMARY KEY, title TEXT NOT NULL, body TEXT NOT NULL, metadata TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS revisions(note_id INTEGER NOT NULL, revision INTEGER NOT NULL, body TEXT NOT NULL, PRIMARY KEY(note_id, revision));
CREATE TABLE IF NOT EXISTS tags(note_id INTEGER NOT NULL, tag TEXT NOT NULL, PRIMARY KEY(note_id, tag));
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(title, body, content='notes', content_rowid='id');`)
	must(err)
	tx, err := db.Begin()
	must(err)
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO notes(id,title,body,metadata,updated_at) VALUES(?,?,?,?,?)`)
	must(err)
	for i := 1; i <= notes; i++ {
		meta, _ := json.Marshal(map[string]any{"source": "synthetic", "rank": i, "flags": []string{"offline", "indexed"}})
		_, err = stmt.Exec(i, fmt.Sprintf("Notebook note %04d", i), fmt.Sprintf("notrios synthetic body for note %04d with searchable evidence", i), string(meta), i)
		must(err)
	}
	stmt.Close()
	must(tx.Commit())
	_, err = db.Exec(`INSERT INTO notes_fts(notes_fts) VALUES('rebuild'); INSERT OR REPLACE INTO revisions(note_id,revision,body) VALUES(1,1,'initial body'); INSERT OR REPLACE INTO tags(note_id,tag) VALUES(1,'synthetic');`)
	must(err)
	var count int
	must(db.QueryRow(`SELECT count(*) FROM notes_fts WHERE notes_fts MATCH 'searchable'`).Scan(&count))
	var jsonValue string
	must(db.QueryRow(`SELECT json_extract(metadata,'$.source') FROM notes WHERE id=1`).Scan(&jsonValue))
	var journal string
	must(db.QueryRow(`PRAGMA journal_mode`).Scan(&journal))
	var integrity string
	must(db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity))
	if count != notes || jsonValue != "synthetic" || journal != "wal" || integrity != "ok" {
		panic("workload assertions failed")
	}
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	must(err)
	// Reopen is performed by the shell runner; this process leaves a valid checkpointed file.
	result := map[string]any{"candidate": "modernc", "sqlite": "3.53.3 (modernc v1.57.0)", "notes": notes, "fts_matches": count, "json_source": jsonValue, "journal_mode": journal, "integrity": integrity, "elapsed_ms": time.Since(start).Milliseconds()}
	b, _ := json.Marshal(result)
	fmt.Println(string(b))
}
