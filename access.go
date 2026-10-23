package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Additional hosts are an explicit private-network opt-in. Authentication is
// optional; without it, access is controlled by the deployment network.
func (a *App) configureAccess(passwordFile, hosts string) error {
	allowed := make(map[string]bool)
	for _, entry := range strings.Split(hosts, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		host, port, err := net.SplitHostPort(entry)
		p, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || p < 1 || p > 65535 || host == "" || strings.ContainsAny(host, "/\\@% \t\r\n") {
			return errors.New("allowed hosts must be explicit host:port entries, without URLs or wildcards")
		}
		if net.ParseIP(host) == nil && !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`).MatchString(host) {
			return errors.New("invalid allowed hostname")
		}
		allowed[entry] = true
	}
	if passwordFile == "" {
		a.allowedHosts = allowed
		a.passwordHash = nil
		return nil
	}
	info, err := os.Lstat(passwordFile)
	if err != nil {
		return fmt.Errorf("check access password: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("access password must be a regular file")
	}
	file, err := os.Open(passwordFile)
	if err != nil {
		return fmt.Errorf("open access password: %w", err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("access password must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return fmt.Errorf("read access password: %w", err)
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if len(password) < 16 || len(password) > 256 || strings.ContainsAny(password, "\x00\r\n") {
		return errors.New("access password must contain 16–256 bytes, on one line")
	}
	hash := sha256.Sum256([]byte(password))
	a.passwordHash = &hash
	a.allowedHosts = allowed
	return nil
}

func (a *App) acceptsHost(authority string) bool {
	if a.allowedHosts[strings.ToLower(authority)] {
		return true
	}
	host := authority
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func (a *App) authenticate(w http.ResponseWriter, r *http.Request) bool {
	if a.passwordHash == nil {
		return true
	}
	user, password, ok := r.BasicAuth()
	hash := sha256.Sum256([]byte(password))
	validPassword := subtle.ConstantTimeCompare(hash[:], a.passwordHash[:]) == 1
	if ok && user == "qsobook" && validPassword {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="QSObook", charset="UTF-8"`)
	a.httpError(w, r, "Autenticação necessária", http.StatusUnauthorized)
	return false
}
