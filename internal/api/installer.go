package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/installer"
	"github.com/ryxoui/ryxoui/internal/nodes"
	"github.com/ryxoui/ryxoui/internal/ssh"
)

// InstallerHandler returns the handler of the install paths.
//
// The paths live outside the API, because a machine that is not set up yet
// has to reach them. The two calls that change something need an API token
// with the scope nodes:write.
func (s *Server) InstallerHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /install/ryxoui.sh", s.handleInstallScript)
	mux.HandleFunc("GET /install/ca.pem", s.handleInstallCA)
	mux.HandleFunc("GET /install/ryxoui", s.handleInstallBinary)
	mux.HandleFunc("POST /install/register", s.handleInstallRegister)
	mux.HandleFunc("POST /install/confirm", s.handleInstallConfirm)
	return mux
}

// handleInstallScript serves the installer of a node.
func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(installer.Script)); err != nil {
		s.log.Error("write the install script", "error", err)
	}
}

// handleInstallCA serves the authority of the panel.
func (s *Server) handleInstallCA(w http.ResponseWriter, r *http.Request) {
	cert, err := s.certs.EnsureCA(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(cert.Certificate)); err != nil {
		s.log.Error("write the authority of the panel", "error", err)
	}
}

// handleInstallBinary serves the binary of the panel for a platform.
//
// The artefact of the newest release is used, so a node always gets the same
// build as the panel.
func (s *Server) handleInstallBinary(w http.ResponseWriter, r *http.Request) {
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	if platform == "" {
		platform = "linux/amd64"
	}

	release, url, err := s.artefact(platform)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if url == "" {
		handleError(w, r, s.log, notFound("no artefact for %s", platform))
		return
	}

	archive, err := s.fetch(url)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	binary, err := binaryOf(archive)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=ryxoui")
	w.Header().Set("X-RyxoUI-Version", release)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(binary); err != nil {
		s.log.Error("write the binary", "error", err)
	}
}

// handleInstallRegister registers a machine at the panel and gives it the key
// the panel needs to drive it.
func (s *Server) handleInstallRegister(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.tokenPrincipal(w, r)
	if !ok {
		return
	}
	if !principal.HasScope(authz.ScopeNodesWrite) {
		writeError(w, http.StatusForbidden, CodeForbidden, "the token may not register nodes", nil)
		return
	}

	var body struct {
		Name     string `json:"name"`
		Address  string `json:"address"`
		SSHPort  int    `json:"sshPort"`
		SSHUser  string `json:"sshUser"`
		Remark   string `json:"remark"`
		Template string `json:"templateId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	address := strings.TrimSpace(body.Address)
	if name == "" || address == "" {
		handleError(w, r, s.log, badRequest("the name and the address are required"))
		return
	}
	if body.SSHPort == 0 {
		body.SSHPort = 22
	}
	if body.SSHUser == "" {
		body.SSHUser = "root"
	}

	// Every node gets its own key pair.
	pair, err := ssh.GenerateKeyPair("ryxoui")
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	key, err := s.keys.Create(r.Context(), nodes.SSHKey{
		Name:       name + "-key",
		PublicKey:  pair.PublicKey,
		PrivateKey: pair.PrivateKey,
		Comment:    "ryxoui",
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	node, err := s.nodes.Create(r.Context(), nodes.Node{
		Name:       name,
		Address:    address,
		SSHHost:    address,
		SSHPort:    body.SSHPort,
		SSHUser:    body.SSHUser,
		SSHKeyID:   key.ID,
		Remark:     body.Remark,
		TemplateID: body.Template,
		Enabled:    true,
		Status:     nodes.StatusConnecting,
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateName) {
			handleError(w, r, s.log, conflict("the node name is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           node.ID,
		"name":         node.Name,
		"sshPublicKey": strings.TrimSpace(key.PublicKey),
		"sshHost":      node.SSHHost,
		"sshPort":      node.SSHPort,
		"sshUser":      node.SSHUser,
	})
}

// handleInstallConfirm tells the panel that the key of the panel works on
// the node.
func (s *Server) handleInstallConfirm(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.tokenPrincipal(w, r)
	if !ok {
		return
	}
	if !principal.HasScope(authz.ScopeNodesWrite) {
		writeError(w, http.StatusForbidden, CodeForbidden, "the token may not register nodes", nil)
		return
	}

	var body struct {
		NodeID string `json:"nodeId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.NodeID) == "" {
		handleError(w, r, s.log, badRequest("the node is required"))
		return
	}

	node, err := s.nodes.Get(r.Context(), body.NodeID)
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	node.KeyInstalled = true
	if err := s.nodes.Update(r.Context(), node); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nodeId": node.ID})
}

// tokenPrincipal resolves the caller from an API token alone.
func (s *Server) tokenPrincipal(w http.ResponseWriter, r *http.Request) (*Principal, bool) {
	if bearerToken(r) == "" {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "an api token is required", nil)
		return nil, false
	}

	principal, err := s.resolvePrincipal(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "the api token is not valid", nil)
		return nil, false
	}
	if !principal.IsToken() {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "an api token is required", nil)
		return nil, false
	}
	return principal, true
}

// artefact finds the download of a platform in the newest release of the
// panel.
func (s *Server) artefact(platform string) (string, string, error) {
	platform = strings.ReplaceAll(strings.TrimSpace(platform), "-", "/")
	if platform == "" {
		platform = "linux/amd64"
	}

	releases, err := s.releases.Releases(context.Background())
	if err != nil {
		return "", "", err
	}

	// The artefacts are named ryxoui_<version>_<os>_<arch>.tar.gz, so the
	// platform is looked for in the name.
	parts := strings.Split(platform, "/")
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		for _, asset := range release.Assets {
			name := strings.ToLower(asset.Name)
			if !strings.Contains(name, parts[0]) {
				continue
			}
			if len(parts) > 1 && !strings.Contains(name, strings.ReplaceAll(parts[1], "amd64", "amd64")) {
				continue
			}
			if !strings.HasSuffix(name, ".tar.gz") {
				continue
			}
			return release.TagName, asset.DownloadURL, nil
		}
	}
	return "", "", nil
}

// fetch downloads a file.
func (s *Server) fetch(url string) ([]byte, error) {
	client := s.releases.HTTP
	if client == nil {
		client = &http.Client{}
	}

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "ryxoui-panel")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errors.New("the server answered with " + response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 256<<20))
}

// binaryOf takes the binary of the panel out of its archive.
func binaryOf(archive []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the archive holds no binary")
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := header.Name
		if index := strings.LastIndex(name, "/"); index >= 0 {
			name = name[index+1:]
		}
		if name != "ryxoui" && name != "ryxoui.exe" {
			continue
		}
		return io.ReadAll(io.LimitReader(tarReader, 256<<20))
	}
}
