package wasabihttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	var agent wasabi.Agent

	// must be authenticated
	_, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	togid, err := wasabi.ToGid(id)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	err = wasabi.FetchAgent(togid, &agent)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// if the request comes from intel, just return JSON
	if strings.Contains(req.Referer(), "intel.ingress.com") {
		data, _ := json.MarshalIndent(agent, "", "\t")
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(data))
		return
	}

	// TemplateExecute prints directly to the result writer
	if err := wasabiHTTPSTemplateExecute(res, req, "agent", agent); err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}
