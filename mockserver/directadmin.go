package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// DirectAdmin legacy API mock. GET or POST, HTTP Basic auth (any credentials
// accepted except username "bad" -> 401). Responses use the legacy
// URL-encoded format: `error=0&text=Success&details=...` (error=1 on failure).

type daAccount struct {
	Username  string
	Email     string
	Domain    string
	Package   string
	IP        string
	Password  string
	Suspended bool
	CreatedAt time.Time
}

func writeDA(w http.ResponseWriter, status int, values url.Values) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(values.Encode()))
}

func daResult(errFlag int, text, details string) url.Values {
	return url.Values{
		"error":   {fmt.Sprintf("%d", errFlag)},
		"text":    {text},
		"details": {details},
	}
}

// daAuth enforces Basic auth; returns false after writing the 401 response.
func daAuth(w http.ResponseWriter, r *http.Request) bool {
	user, pass, ok := r.BasicAuth()
	if !ok || user == "" || pass == "" || user == "bad" {
		w.Header().Set("WWW-Authenticate", `Basic realm="DirectAdmin Mock"`)
		writeDA(w, http.StatusUnauthorized, daResult(1, "Login failed", "invalid credentials"))
		return false
	}
	return true
}

// daParam returns the first non-empty form/query value among keys.
func daParam(r *http.Request, keys ...string) string {
	for _, k := range keys {
		if v := r.Form.Get(k); v != "" {
			return v
		}
	}
	return ""
}

// GET|POST /CMD_API_ACCOUNT_USER (action=create)
func (s *Server) handleDAAccountUser(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	action := daParam(r, "action")
	if action != "create" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "unknown or missing action: "+action))
		return
	}
	username := daParam(r, "username", "user")
	if username == "" {
		writeDA(w, http.StatusOK, daResult(1, "Error Creating User", "username is required"))
		return
	}
	if username == "failme" {
		writeDA(w, http.StatusOK, daResult(1, "Error Creating User", "forced failure"))
		return
	}

	s.mu.Lock()
	if _, exists := s.daAccounts[username]; exists {
		s.mu.Unlock()
		writeDA(w, http.StatusOK, daResult(1, "Error Creating User", "That username already exists"))
		return
	}
	s.daAccounts[username] = &daAccount{
		Username:  username,
		Email:     daParam(r, "email"),
		Domain:    daParam(r, "domain"),
		Package:   daParam(r, "package", "pkg"),
		IP:        daParam(r, "ip"),
		Password:  daParam(r, "passwd", "password"),
		CreatedAt: time.Now(),
	}
	s.mu.Unlock()

	writeDA(w, http.StatusOK, daResult(0, "Success", "User "+username+" created"))
}

// GET|POST /CMD_API_SELECT_USERS - suspend/unsuspend via
// select0=<user>&suspend=Suspend|Unsuspend, delete via delete=yes.
func (s *Server) handleDASelectUsers(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()

	var users []string
	for i := 0; ; i++ {
		u := r.Form.Get(fmt.Sprintf("select%d", i))
		if u == "" {
			break
		}
		users = append(users, u)
	}
	if len(users) == 0 {
		writeDA(w, http.StatusOK, daResult(1, "Error", "no users selected"))
		return
	}

	doDelete := daParam(r, "delete") == "yes"
	suspendAction := daParam(r, "suspend", "dosuspend") // "Suspend" | "Unsuspend"
	if !doDelete && suspendAction == "" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "nothing to do: pass suspend=Suspend|Unsuspend or delete=yes"))
		return
	}

	s.mu.Lock()
	for _, u := range users {
		if _, ok := s.daAccounts[u]; !ok {
			s.mu.Unlock()
			writeDA(w, http.StatusOK, daResult(1, "Error", "user "+u+" does not exist"))
			return
		}
	}
	verb := ""
	for _, u := range users {
		switch {
		case doDelete:
			delete(s.daAccounts, u)
			verb = "deleted"
		case suspendAction == "Suspend":
			s.daAccounts[u].Suspended = true
			verb = "suspended"
		case suspendAction == "Unsuspend":
			s.daAccounts[u].Suspended = false
			verb = "unsuspended"
		default:
			s.mu.Unlock()
			writeDA(w, http.StatusOK, daResult(1, "Error", "invalid suspend value: "+suspendAction))
			return
		}
	}
	s.mu.Unlock()

	writeDA(w, http.StatusOK, daResult(0, "Success", fmt.Sprintf("%d user(s) %s", len(users), verb)))
}

// GET|POST /CMD_API_MODIFY_USER (action=package)
func (s *Server) handleDAModifyUser(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	if action := daParam(r, "action"); action != "package" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "unknown or missing action: "+action))
		return
	}
	user := daParam(r, "user", "username")
	pkg := daParam(r, "package", "pkg")
	if user == "" || pkg == "" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "user and package are required"))
		return
	}

	s.mu.Lock()
	acct, ok := s.daAccounts[user]
	if ok {
		acct.Package = pkg
	}
	s.mu.Unlock()
	if !ok {
		writeDA(w, http.StatusOK, daResult(1, "Error", "user "+user+" does not exist"))
		return
	}
	writeDA(w, http.StatusOK, daResult(0, "Success", "Package changed to "+pkg))
}

// daPkgParams collects the package limit fields (and unlimited flags) from the
// request form so tests can introspect what the adapter sent.
func daPkgParams(r *http.Request) map[string]string {
	out := map[string]string{}
	for _, k := range []string{
		"bandwidth", "quota", "vdomains", "nsubdomains", "domainptr", "nemails", "mysql", "ftp",
		"ubandwidth", "uquota", "uvdomains", "unsubdomains", "udomainptr", "unemails", "umysql", "uftp",
		"cgi", "ssh", "userssh", "clamav",
	} {
		if v := r.Form.Get(k); v != "" {
			out[k] = v
		}
	}
	return out
}

// GET|POST /CMD_API_MANAGE_USER_PACKAGES (action=create|modify|delete) - on-the-fly
// user packages for dynamic/custom-spec products.
func (s *Server) handleDAManageUserPackages(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	switch action := daParam(r, "action"); action {
	case "create":
		name := daParam(r, "packagename", "package")
		if name == "" {
			writeDA(w, http.StatusOK, daResult(1, "Error", "packagename is required"))
			return
		}
		s.mu.Lock()
		if _, exists := s.daPackages[name]; exists {
			s.mu.Unlock()
			writeDA(w, http.StatusOK, daResult(1, "Error", "That package already exists"))
			return
		}
		s.daPackages[name] = &panelPackage{Name: name, Params: daPkgParams(r), CreatedAt: time.Now()}
		s.mu.Unlock()
		writeDA(w, http.StatusOK, daResult(0, "Success", "Package "+name+" created"))
	case "modify":
		name := daParam(r, "packagename", "package")
		s.mu.Lock()
		pkg, ok := s.daPackages[name]
		if ok {
			pkg.Params = daPkgParams(r)
		}
		s.mu.Unlock()
		if !ok {
			writeDA(w, http.StatusOK, daResult(1, "Error", "package does not exist"))
			return
		}
		writeDA(w, http.StatusOK, daResult(0, "Success", "Package "+name+" modified"))
	case "delete":
		name := daParam(r, "select0", "packagename", "package")
		if name == "" {
			writeDA(w, http.StatusOK, daResult(1, "Error", "select0 is required"))
			return
		}
		s.mu.Lock()
		_, ok := s.daPackages[name]
		delete(s.daPackages, name)
		s.mu.Unlock()
		if !ok {
			writeDA(w, http.StatusOK, daResult(1, "Error", "package does not exist"))
			return
		}
		writeDA(w, http.StatusOK, daResult(0, "Success", "Package "+name+" deleted"))
	default:
		writeDA(w, http.StatusOK, daResult(1, "Error", "unknown or missing action: "+action))
	}
}

// GET /CMD_API_PACKAGES_USER - lists configured package names (read-only), or,
// when a `package=<name>` query param is given, returns that one package's
// full raw field set instead (real DirectAdmin overloads this same command -
// used by directadmin.Client.EnsurePackage to clone a TemplatePackage's
// long-tail settings). Real DirectAdmin repeats the literal key "list[]" for
// each name in list mode (confirmed against a real server:
// forum.directadmin.com/threads/whmcs-directadmin-packages) and, like its
// sibling dump-style endpoints, carries no `error` key on success in either
// mode - a missing single package is the one exception, reported as error=1.
func (s *Server) handleDAPackagesUser(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	if name := daParam(r, "package"); name != "" {
		s.mu.Lock()
		pkg, ok := s.daPackages[name]
		s.mu.Unlock()
		if !ok {
			writeDA(w, http.StatusOK, daResult(1, "Error", "Package "+name+" does not exist"))
			return
		}
		vals := url.Values{"packagename": {pkg.Name}}
		for k, v := range pkg.Params {
			vals.Set(k, v)
		}
		writeDA(w, http.StatusOK, vals)
		return
	}
	s.mu.Lock()
	names := make([]string, 0, len(s.daPackages))
	for n := range s.daPackages {
		names = append(names, n)
	}
	sort.Strings(names)
	s.mu.Unlock()
	vals := url.Values{}
	for _, n := range names {
		vals.Add("list[]", n)
	}
	writeDA(w, http.StatusOK, vals)
}

// GET|POST /CMD_API_USER_PASSWD
func (s *Server) handleDAUserPasswd(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	user := daParam(r, "username", "user")
	passwd := daParam(r, "passwd", "password")
	passwd2 := daParam(r, "passwd2")
	if user == "" || passwd == "" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "username and passwd are required"))
		return
	}
	if passwd2 != "" && passwd2 != passwd {
		writeDA(w, http.StatusOK, daResult(1, "Error", "Passwords do not match"))
		return
	}

	s.mu.Lock()
	acct, ok := s.daAccounts[user]
	if ok {
		acct.Password = passwd
	}
	s.mu.Unlock()
	if !ok {
		writeDA(w, http.StatusOK, daResult(1, "Error", "user "+user+" does not exist"))
		return
	}
	writeDA(w, http.StatusOK, daResult(0, "Success", "Password changed"))
}

// GET /CMD_API_SHOW_USERS - read-only user listing used by the admin
// "Test Connection" probe. Returns the legacy `list[]=user` array; auth alone
// (not any account) is what the probe validates.
func (s *Server) handleDAShowUsers(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	s.mu.Lock()
	list := url.Values{}
	for name := range s.daAccounts {
		list.Add("list[]", name)
	}
	s.mu.Unlock()
	writeDA(w, http.StatusOK, list)
}

// GET|POST /CMD_API_SHOW_USER_CONFIG - URL-encoded key=value dump.
func (s *Server) handleDAShowUserConfig(w http.ResponseWriter, r *http.Request) {
	if !daAuth(w, r) {
		return
	}
	_ = r.ParseForm()
	user := daParam(r, "user", "username")
	if user == "" {
		writeDA(w, http.StatusOK, daResult(1, "Error", "user is required"))
		return
	}

	s.mu.Lock()
	acct, ok := s.daAccounts[user]
	var cfg url.Values
	if ok {
		suspended := "no"
		if acct.Suspended {
			suspended = "yes"
		}
		cfg = url.Values{
			"username":     {acct.Username},
			"email":        {acct.Email},
			"domain":       {acct.Domain},
			"package":      {acct.Package},
			"ip":           {acct.IP},
			"suspended":    {suspended},
			"date_created": {acct.CreatedAt.UTC().Format("2006-01-02 15:04:05")},
		}
	}
	s.mu.Unlock()
	if !ok {
		writeDA(w, http.StatusOK, daResult(1, "Error", "User "+user+" does not exist"))
		return
	}
	writeDA(w, http.StatusOK, cfg)
}
