package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxUpload = 64 << 20

type changeStamp struct{ Local, External int64 }
type transferPreview struct {
	Key, Kind, Path string
	Expires         time.Time
	Stamp           changeStamp
	Summary         NotebookSummary
	Import          *ADIFPreview
}
type transferState struct {
	sync.Mutex
	Pending *transferPreview
}

func sqliteFileURI(path string) string { u := url.URL{Scheme: "file", Path: path}; return u.String() }
func (s *Store) changeStamp(ctx context.Context) (changeStamp, error) {
	var stamp changeStamp
	err := s.db.QueryRowContext(ctx, "SELECT total_changes(),data_version FROM pragma_data_version").Scan(&stamp.Local, &stamp.External)
	return stamp, err
}
func (a *App) clearTransfer() {
	if a.transfers.Pending != nil {
		if a.transfers.Pending.Path != "" {
			os.RemoveAll(filepath.Dir(a.transfers.Pending.Path))
		}
		a.transfers.Pending = nil
	}
}
func (a *App) closeTransfers() { a.transfers.Lock(); defer a.transfers.Unlock(); a.clearTransfer() }
func (a *App) dataPage(w http.ResponseWriter, r *http.Request) {
	a.transfers.Lock()
	if a.transfers.Pending != nil && time.Now().After(a.transfers.Pending.Expires) {
		a.clearTransfer()
	}
	a.transfers.Unlock()
	n, err := a.store.summary(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	backups, err := a.recoveryFiles()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, Page{Title: "Dados", View: "data", Summary: n, RecoveryFiles: backups, Message: r.URL.Query().Get("message")})
}
func (a *App) downloadBackup(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp(a.store.dataDirectory(), ".qsobook-backup-")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "notebook.db")
	if err = a.store.backup(ctx, path); err == nil {
		err = validateBackup(ctx, path)
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.sendFile(w, r, path, "qsobook-"+time.Now().UTC().Format("20060102T150405Z")+".db", "application/vnd.sqlite3")
}
func (a *App) sendFile(w http.ResponseWriter, r *http.Request, path, name, contentType string) {
	f, err := os.Open(path)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, name, info.ModTime(), f)
}
func (a *App) previewRestore(w http.ResponseWriter, r *http.Request) {
	a.previewUpload(w, r, "restore")
}
func (a *App) previewImport(w http.ResponseWriter, r *http.Request) { a.previewUpload(w, r, "import") }
func (a *App) previewUpload(w http.ResponseWriter, r *http.Request, kind string) {
	a.transfers.Lock()
	defer a.transfers.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	file, _, err := r.FormFile("file")
	if err != nil {
		a.httpError(w, r, "Selecione um arquivo válido.", 400)
		return
	}
	defer file.Close()
	dir, err := os.MkdirTemp(a.store.dataDirectory(), ".qsobook-transfer-")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, "notebook.db")
	if kind == "import" {
		path = filepath.Join(dir, "contacts.adi")
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	size, copyErr := io.Copy(out, io.LimitReader(file, maxUpload+1))
	err = copyErr
	closeErr := out.Close()
	if err != nil || closeErr != nil || size > maxUpload {
		a.fail(w, r, fmt.Errorf("upload failed"))
		return
	}
	key, err := newSubmissionKey()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p := &transferPreview{Kind: kind, Key: key, Path: path, Expires: time.Now().Add(15 * time.Minute)}
	if kind == "restore" {
		if err = validateBackup(ctx, path); err != nil {
			a.httpError(w, r, errBackupInvalid.Error(), 400)
			return
		}
		staged, e := openStore(ctx, path)
		if e != nil {
			a.httpError(w, r, errBackupInvalid.Error(), 400)
			return
		}
		p.Summary, err = staged.summary(ctx)
		staged.db.Close()
	} else {
		p.Import, err = a.store.previewADIF(ctx, path)
	}
	if err != nil {
		if errors.Is(err, errADIFInvalid) {
			a.httpError(w, r, a.translate(requestLanguage(r), errADIFInvalid.Error())+strings.TrimPrefix(err.Error(), errADIFInvalid.Error()), 400)
		} else {
			a.fail(w, r, err)
		}
		return
	}
	p.Stamp, err = a.store.changeStamp(ctx)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.clearTransfer()
	a.transfers.Pending = p
	keep = true
	a.render(w, r, Page{Title: "Revise antes de confirmar", View: "transfer", Action: "/data", Transfer: p})
}
func (a *App) confirmTransfer(w http.ResponseWriter, r *http.Request) {
	a.transfers.Lock()
	defer a.transfers.Unlock()
	p := a.transfers.Pending
	key := r.PostForm.Get("key")
	if p == nil || time.Now().After(p.Expires) || subtle.ConstantTimeCompare([]byte(key), []byte(p.Key)) != 1 {
		a.httpError(w, r, "Prévia expirada. Selecione o arquivo novamente.", 409)
		return
	}
	stamp, err := a.store.changeStamp(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if stamp != p.Stamp {
		a.httpError(w, r, "Os dados mudaram desde a prévia. Selecione o arquivo novamente.", 409)
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		a.httpError(w, r, "Confirme a operação antes de continuar.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if p.Kind == "restore" {
		replacementToken, e := newSubmissionKey()
		if e != nil {
			a.fail(w, r, e)
			return
		}
		// Create and validate a durable recovery snapshot before any replacement.
		dir := filepath.Join(a.store.dataDirectory(), "backups")
		if err = os.MkdirAll(dir, 0700); err != nil {
			a.fail(w, r, err)
			return
		}
		recoveryKey, e := newSubmissionKey()
		if e != nil {
			a.fail(w, r, e)
			return
		}
		recovery := filepath.Join(dir, "pre-restore-"+time.Now().UTC().Format("20060102T150405Z")+"-"+recoveryKey+".db")
		if err = a.store.backup(ctx, recovery); err == nil {
			err = validateBackup(ctx, recovery)
		}
		if err == nil {
			err = a.store.restore(ctx, p.Path)
		}
		if err != nil {
			a.fail(w, r, err)
			return
		}
		// Old forms must never modify IDs belonging to the restored notebook.
		a.token = replacementToken
	} else {
		if _, err = a.store.importADIF(ctx, p.Import); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	a.clearTransfer()
	redirect(w, r, dataMessage("Operação concluída. Os dados foram salvos."))
}

var recoveryName = regexp.MustCompile(`^pre-restore-[0-9]{8}T[0-9]{6}Z-[a-f0-9]{32}\.db$`)

func (a *App) recoveryFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(a.store.dataDirectory(), "backups"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if recoveryName.MatchString(e.Name()) && e.Type().IsRegular() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
func (a *App) downloadRecovery(w http.ResponseWriter, r *http.Request) {
	name := r.PostForm.Get("name")
	if !recoveryName.MatchString(name) || strings.ContainsAny(name, "/\\") {
		a.httpError(w, r, "Arquivo inválido", 400)
		return
	}
	path := filepath.Join(a.store.dataDirectory(), "backups", name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		a.httpError(w, r, "Registro não encontrado", 404)
		return
	}
	a.sendFile(w, r, path, name, "application/vnd.sqlite3")
}

func (a *App) cancelTransfer(w http.ResponseWriter, r *http.Request) {
	a.transfers.Lock()
	defer a.transfers.Unlock()
	p := a.transfers.Pending
	if p != nil && subtle.ConstantTimeCompare([]byte(r.PostForm.Get("key")), []byte(p.Key)) == 1 {
		a.clearTransfer()
	}
	redirect(w, r, "/data")
}
