package wasabeehttps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/wasabee-project/Wasabee-Server"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func setupRouter() *mux.Router {
	// Main Router
	router := wasabee.NewRouter()

	// apply to all
	router.Use(headersMW)
	router.Use(scannerMW)
	// router.Use(logRequestMW)
	// router.Use(debugMW)
	router.Use(config.unrolled.Handler)
	router.NotFoundHandler = http.HandlerFunc(notFoundRoute)
	router.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Google Oauth2 stuff (constants defined in server.go)
	router.HandleFunc(login, googleRoute).Methods("GET")
	router.HandleFunc(callback, callbackRoute).Methods("GET")
	router.HandleFunc(aptoken, apTokenRoute).Methods("POST")
	// common files that live under /static
	router.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	router.Path("/robots.txt").Handler(http.RedirectHandler("/static/robots.txt", http.StatusFound))
	router.Path("/sitemap.xml").Handler(http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	router.Path("/.well-known/security.txt").Handler(http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))
	// this cannot be a redirect -- sent it raw
	router.HandleFunc("/firebase-messaging-sw.js", fbmswRoute).Methods("GET")
	// do not make these static -- they should be translated via the templates system
	router.HandleFunc("/privacy", privacyRoute).Methods("GET")
	router.HandleFunc("/", frontRoute).Methods("GET")

	// /api/v1/... route
	api := wasabee.Subrouter(apipath)
	api.Methods("OPTIONS").HandlerFunc(optionsRoute)
	setupAuthRoutes(api)
	api.Use(authMW)
	api.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /me route
	me := wasabee.Subrouter(me)
	me.Methods("OPTIONS").HandlerFunc(optionsRoute)
	me.HandleFunc("", meShowRoute).Methods("GET")
	me.Use(authMW)
	me.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	// /rocks route -- why a subrouter? for JSON error messages
	rocks := wasabee.Subrouter("/rocks")
	rocks.HandleFunc("", rocksCommunityRoute).Methods("POST")
	rocks.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /static files
	static := wasabee.Subrouter("/static")
	static.PathPrefix("/").Handler(http.FileServer(http.Dir(config.FrontendPath)))
	// static.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	return router
}

// implied /api/v1
func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	r.HandleFunc("/draw", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", pDrawGetRoute).Methods("GET", "HEAD")
	r.HandleFunc("/draw/{document}", pDrawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", pDrawUpdateRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}/delete", pDrawDeleteRoute).Methods("GET", "DELETE")
	r.HandleFunc("/draw/{document}/chown", pDrawChownRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/draw/{document}/stock", pDrawStockRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/order", pDrawOrderRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/info", pDrawInfoRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/stat", pDrawStatRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/perms", pDrawPermsRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/perms", pDrawPermsAddRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/perms", pDrawPermsDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}/delperm", pDrawPermsDeleteRoute).Methods("GET") // .Queries("team", "{team}", "role", "{role}")
	r.HandleFunc("/draw/{document}/myroute", pDrawMyRouteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/copy", pDrawCopyRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/assign", pDrawLinkAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/color", pDrawLinkColorRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/desc", pDrawLinkDescRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/complete", pDrawLinkCompleteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/incomplete", pDrawLinkIncompleteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/swap", pDrawLinkSwapRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/marker/{marker}/assign", pDrawMarkerAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/comment", pDrawMarkerCommentRoute).Methods("POST")
	// agent acknowledge the assignment
	r.HandleFunc("/draw/{document}/marker/{marker}/acknowledge", pDrawMarkerAcknowledgeRoute).Methods("GET")
	// agent mark as complete
	r.HandleFunc("/draw/{document}/marker/{marker}/complete", pDrawMarkerCompleteRoute).Methods("GET")
	// agent undo complete mark
	r.HandleFunc("/draw/{document}/marker/{marker}/incomplete", pDrawMarkerIncompleteRoute).Methods("GET")
	// operator verify completing
	r.HandleFunc("/draw/{document}/marker/{marker}/reject", pDrawMarkerRejectRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/portal/{portal}/comment", pDrawPortalCommentRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/hardness", pDrawPortalHardnessRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/keyonhand", pDrawPortalKeysRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}", pDrawPortalRoute).Methods("GET")

	// manual location post
	r.HandleFunc("/me", meSetAgentLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}")
	// -- do not use, just here for safety
	r.HandleFunc("/me", meShowRoute).Methods("GET")
	// r.HandleFunc("/me/delete", meDeleteRoute).Methods("GET") // purge all info for a agent
	// toggle RAID/JEAH polling
	r.HandleFunc("/me/settings", meSettingsRoute).Methods("GET")
	r.HandleFunc("/me/operations", meOperationsRoute).Methods("GET")
	r.HandleFunc("/me/statuslocation", meStatusLocationRoute).Methods("GET").Queries("sl", "{sl}")
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")
	r.HandleFunc("/me/logout", meLogoutRoute).Methods("GET")
	r.HandleFunc("/me/firebase", meFirebaseRoute).Methods("POST")

	// other agents
	// "profile" page, such as it is
	r.HandleFunc("/agent/{id}", agentProfileRoute).Methods("GET")
	r.HandleFunc("/agent/{id}/image", agentPictureRoute).Methods("GET")
	// send a message to a agent
	r.HandleFunc("/agent/{id}/message", agentMessageRoute).Methods("POST")
	r.HandleFunc("/agent/{id}/target", agentTargetRoute).Methods("POST")

	// teams
	// create a new team
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")
	r.HandleFunc("/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}")
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE")
	r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET", "DELETE")
	r.HandleFunc("/team/{team}/chown", chownTeamRoute).Methods("GET").Queries("to", "{to}")
	// GUI to do basic edit (owner)
	r.HandleFunc("/team/{team}/edit", editTeamRoute).Methods("GET")
	// (re)import the team from rocks
	r.HandleFunc("/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")
	// configure team link to enl.rocks community
	r.HandleFunc("/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}")
	// broadcast a message to the team
	r.HandleFunc("/team/{team}/announce", announceTeamRoute).Methods("POST")
	r.HandleFunc("/team/{team}/{key}", addAgentToTeamRoute).Methods("GET", "POST")
	r.HandleFunc("/team/{team}/{gid}/squad", setAgentTeamSquadRoute).Methods("POST")
	r.HandleFunc("/team/{team}/{gid}/displayname", setAgentTeamDisplaynameRoute).Methods("POST")
	r.HandleFunc("/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")

	r.HandleFunc("/d", getDefensiveKeys).Methods("GET")
	r.HandleFunc("/d", setDefensiveKey).Methods("POST")

	// server control functions
	// trigger the server refresh of the template files
	r.HandleFunc("/templates/refresh", templateUpdateRoute).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
}

// display the front page
func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "index", nil)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// display the privacy policy
func privacyRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "privacy", nil)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// this just reloads the templates on disk ; if someone makes a change we don't need to restart the server
func templateUpdateRoute(res http.ResponseWriter, req *http.Request) {
	var err error
	config.TemplateSet, err = wasabee.TemplateConfig(config.FrontendPath) // XXX KLUDGE FOR NOW -- this does not update the other protocols
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "Templates reloaded")
}

// called when a resource/endpoint is not found
func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	i, ok := config.scanners[req.RemoteAddr]
	if ok {
		config.scanners[req.RemoteAddr] = i + 1
	} else {
		config.scanners[req.RemoteAddr] = 1
	}
	wasabee.Log.Debugf("404: %s", req.URL)
	http.Error(res, "404: no light here.", http.StatusNotFound)
}

// called when a resource/endpoint is not found
func notFoundJSONRoute(res http.ResponseWriter, req *http.Request) {
	err := fmt.Errorf("not found")
	wasabee.Log.Debug("not found: %s", req.URL)
	i, ok := config.scanners[req.RemoteAddr]
	if ok {
		config.scanners[req.RemoteAddr] = i + 1
	} else {
		config.scanners[req.RemoteAddr] = 1
	}
	wasabee.Log.Debugf("404: %s", req.URL)
	http.Error(res, jsonError(err), http.StatusNotFound)
}

func fbmswRoute(res http.ResponseWriter, req *http.Request) {
	prefix := http.Dir(config.FrontendPath)
	http.ServeFile(res, req, fmt.Sprintf("%s/static/firebase/firebase-messaging-sw.js", prefix))
}

// final step of the oauth cycle
func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type googleData struct {
		Gid   wasabee.GoogleID `json:"id"`
		Name  string           `json:"name"`
		Email string           `json:"email"`
		Pic   string           `json:"picture"`
	}

	content, err := getAgentInfo(req.Context(), req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		wasabee.Log.Notice(err)
		return
	}
	// wasabee.Log.Debug(string(content))

	var m googleData
	if err = json.Unmarshal(content, &m); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// session cookie
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		wasabee.Log.Notice("Cookie error: ", err)
		ses = sessions.NewSession(config.store, config.sessionName)
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,
			SameSite: http.SameSiteNoneMode, // requires go 1.13
			Secure:   true,
		}
		// don't stomp on err since we are currently in an error path
		if saveerr := ses.Save(req, res); saveerr != nil {
			wasabee.Log.Notice(saveerr)
			http.Error(res, saveerr.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	location := me + "?a=0"
	if ses.Values["loginReq"] != nil {
		rr := ses.Values["loginReq"].(string)
		if rr[:len(me)] == me || rr[:len(login)] == login { // leave /me check in place
			location = me + "?postlogin=1"
		} else {
			location = rr
		}
		delete(ses.Values, "loginReq")
	}

	authorized, err := m.Gid.InitAgent() // V & .rocks authorization takes place here
	if !authorized {
		http.Error(res, "Smurf go away!", http.StatusUnauthorized)
		return
	}
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	err = m.Gid.UpdatePicture(m.Pic)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:     "/",
		SameSite: http.SameSiteNoneMode, // requires go 1.13
		Secure:   true,
	}

	_ = ses.Save(req, res)
	iname, err := m.Gid.IngressName()
	if err != nil {
		wasabee.Log.Debug("no iname at end of login? %n", m.Gid)
	}
	wasabee.Log.Infof("%s login", iname)
	http.Redirect(res, req, location, http.StatusFound)
}

// the secret value exchanged / verified each request
// not really a nonce, but it started life as one
func calculateNonce(gid wasabee.GoogleID) (string, string) {
	t := time.Now()
	now := t.Round(time.Hour).String()
	prev := t.Add(0 - time.Hour).Round(time.Hour).String()
	// something specific to the agent, something secret, something short-term
	current := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, now)))
	previous := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, prev)))
	return hex.EncodeToString(current[:]), hex.EncodeToString(previous[:])
}

// read the result from provider at end of oauth session
func getAgentInfo(rctx context.Context, state string, code string) ([]byte, error) {
	if state != config.oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}

	ctx, cancel := context.WithTimeout(rctx, wasabee.GetTimeout(5*time.Second))
	defer cancel()
	token, err := config.OauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	cancel()

	contents, err := getOauthUserInfo(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting agent info: %s", err.Error())
	}

	return contents, nil
}

// used in getAgentInfo and apTokenRoute -- takes a user's Oauth2 token and requests their info
func getOauthUserInfo(accessToken string) ([]byte, error) {
	url := config.OauthUserInfoURL

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		wasabee.Log.Error(err)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	client := &http.Client{
		Timeout: wasabee.GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabee.Log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabee.Log.Error(err)
		return nil, err
	}
	return body, nil
}

// read the gid from the session cookie and return it
// this is the primary way to ensure a agent is authenticated
func getAgentID(req *http.Request) (wasabee.GoogleID, error) {
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		return "", err
	}

	// XXX I think this is impossible to trigger now
	if ses.Values["id"] == nil {
		err := errors.New("getAgentID called for unauthenticated agent")
		wasabee.Log.Critical(err)
		return "", err
	}

	var agentID = wasabee.GoogleID(ses.Values["id"].(string))
	return agentID, nil
}

// apTokenRoute receives a Google Oauth2 token from the Android/iOS app and sets the authentication cookie
func apTokenRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	// fetched from google
	type googleData struct {
		Gid   wasabee.GoogleID `json:"id"`
		Name  string           `json:"name"`
		Email string           `json:"email"`
		Pic   string           `json:"picture"`
	}
	var m googleData

	// passed in from Android/iOS app
	type token struct {
		AccessToken string `json:"accessToken"`
	}
	var t token

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		wasabee.Log.Notice(err)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		wasabee.Log.Notice(err)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)
	if err = json.Unmarshal(jRaw, &t); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contents, err := getOauthUserInfo(t.AccessToken)
	if err != nil {
		err = fmt.Errorf("failed getting agent info: %s", err.Error())
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = json.Unmarshal(contents, &m); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// session cookie
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		wasabee.Log.Notice("Cookie error: ", err)
		ses = sessions.NewSession(config.store, config.sessionName)
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,                    // force delete
			SameSite: http.SameSiteNoneMode, // requires go 1.13
			Secure:   true,
		}
		_ = ses.Save(req, res)
		res.Header().Set("Connection", "close")
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	authorized, err := m.Gid.InitAgent() // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied: %s", err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	if err != nil { // XXX if !authorized err will be set ; if err is set !authorized ... this is redundant
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:     "/",
		SameSite: http.SameSiteNoneMode, // requires go 1.13
		Secure:   true,
	}
	err = ses.Save(req, res)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	iname, err := m.Gid.IngressName()
	if err != nil {
		wasabee.Log.Error(err)
	}

	var ud wasabee.AgentData
	if err = m.Gid.GetAgentData(&ud); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	data, _ := json.Marshal(ud)

	// cookie := res.Header().Get("set-cookie")
	// wasabee.Log.Debugf("Sending Cookie: %s", cookie);
	res.Header().Set("Connection", "close") // no keep-alives so cookies get processed, does this work in HTTP/2?
	// https://go-review.googlesource.com/c/net/+/121415/
	res.Header().Set("Cache-Control", "no-store")
	wasabee.Log.Infof("%s app login", iname)
	fmt.Fprint(res, string(data))
}
