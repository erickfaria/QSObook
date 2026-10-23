package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

type TrashItem struct {
	ID                              int64
	Kind, Callsign, Name, DeletedAt string
}

func trashTable(kind string) (string, error) {
	switch kind {
	case "station":
		return "stations", nil
	case "contact":
		return "qsos", nil
	}
	return "", errors.New("Tipo inválido")
}
func (s *Store) trashItems(ctx context.Context) ([]TrashItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,'station',callsign,name,deleted_at FROM stations WHERE deleted_at IS NOT NULL UNION ALL SELECT id,'contact',callsign,name,deleted_at FROM qsos WHERE deleted_at IS NOT NULL ORDER BY 5 DESC,2,1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TrashItem{}
	for rows.Next() {
		var item TrashItem
		if err := rows.Scan(&item.ID, &item.Kind, &item.Callsign, &item.Name, &item.DeletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) moveTrash(ctx context.Context, kind string, id int64, restore bool) error {
	table, err := trashTable(kind)
	if err != nil {
		return err
	}
	var value any = time.Now().UTC().Format(time.RFC3339Nano)
	if restore {
		value = nil
	}
	// Repeating an action is harmless; a QSO and its station are independent.
	result, err := s.db.ExecContext(ctx, "UPDATE "+table+" SET deleted_at=? WHERE id=?", value, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}
func (a *App) trash(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.trashItems(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Lixeira", View: "trash", Trash: items, Message: r.URL.Query().Get("message")})
}
func (a *App) trashConfirm(w http.ResponseWriter, r *http.Request) {
	kind := r.FormValue("kind")
	table, err := trashTable(kind)
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	var item TrashItem
	item.ID = id
	item.Kind = kind
	if err = a.store.db.QueryRowContext(r.Context(), "SELECT callsign,name FROM "+table+" WHERE id=? AND deleted_at IS NULL", id).Scan(&item.Callsign, &item.Name); err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Mover para a lixeira", View: "trash-confirm", Trash: []TrashItem{item}})
}
func (a *App) trashAction(w http.ResponseWriter, r *http.Request) {
	id, err := identifier(r, "id")
	if err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	kind := r.PostForm.Get("kind")
	if _, err := trashTable(kind); err != nil {
		a.httpError(w, r, err.Error(), 400)
		return
	}
	action := r.PostForm.Get("action")
	if action != "delete" && action != "restore" {
		a.httpError(w, r, "Ação inválida", 400)
		return
	}
	if err = a.store.moveTrash(r.Context(), kind, id, action == "restore"); err != nil {
		a.fail(w, r, err)
		return
	}
	redirect(w, r, "/trash")
}
