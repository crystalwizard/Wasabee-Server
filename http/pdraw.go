package wasabihttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func pDrawUploadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	// wasabi.Log.Debugf("sent json:", string(jRaw))
	if err = wasabi.PDrawInsert(jRaw, gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawGetRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var o wasabi.Operation
	o.ID = wasabi.OperationID(id)
	if err = o.Populate(gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var newer bool
	ims := req.Header.Get("If-Modified-Since")
	if ims != "" {
		// XXX use http.ParseTime?
		d, err := time.Parse(time.RFC1123, ims)
		if err != nil {
			wasabi.Log.Error(err)
		} else {
			wasabi.Log.Debug("if-modified-since: %s", d)
			m, err := time.Parse("2006-01-02 15:04:05", o.Modified)
			if err != nil {
				wasabi.Log.Error(err)
			} else if d.Before(m) {
				newer = true
			}
		}
	}

	method := req.Header.Get("Method")
	if newer && method == "HEAD" {
		wasabi.Log.Debug("HEAD with 302")
		res.Header().Set("Content-Type", "")          // disable the default output
		http.Redirect(res, req, "", http.StatusFound) // XXX redirect to nothing?
		return
	}

	// JSON if referer is intel.ingress.com
	if strings.Contains(req.Referer(), "intel.ingress.com") {
		res.Header().Set("Content-Type", jsonType)
		s, err := json.MarshalIndent(o, "", "\t")
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(res, string(s))
		return
	}

	// pretty output for everyone else
	friendly, err := pDrawFriendlyNames(&o)
	if err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	if err = templateExecute(res, req, "opdata", friendly); err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func pDrawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = fmt.Errorf("deleting operation %s", op.ID)
		wasabi.Log.Notice(err)
		err := op.Delete()
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can delete an operation")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func jsonError(e error) string {
	s, _ := json.MarshalIndent(e, "", "\t")
	return string(s)
}

func pDrawUpdateRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabi.Log.Debug(string(jBlob))
	if err = wasabi.PDrawUpdate(id, jRaw, gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChownRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := vars["to"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chown(gid, to)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation ownership ")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChgrpRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := wasabi.TeamID(vars["team"])

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chgrp(gid, to)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation team")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawStockRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var o wasabi.Operation
	o.ID = wasabi.OperationID(id)
	if err = o.Populate(gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	url := "https://intel.ingress.com/intel/?z=13&ll="

	type latlon struct {
		lat string
		lon string
	}

	var portals = make(map[wasabi.PortalID]latlon)

	for _, p := range o.OpPortals {
		var l latlon
		l.lat = p.Lat
		l.lon = p.Lon
		portals[p.ID] = l
	}

	var notfirst bool
	for _, l := range o.Links {
		x := portals[l.From]
		if notfirst {
			url += "_"
		} else {
			url += x.lat + "," + x.lon + "&pls="
			notfirst = true
		}
		url += x.lat + "," + x.lon + ","
		y := portals[l.To]
		url += y.lat + "," + y.lon
	}

	// wasabi.Log.Debugf("redirecting to :%s", url)
	http.Redirect(res, req, url, http.StatusFound)
}

type pdrawFriendly struct {
	ID       wasabi.OperationID
	Name     string
	Agent    string
	Color    string
	Modified string
	TeamID   wasabi.TeamID
	Team     string
	Links    []friendlyLink
	Markers  []friendlyMarker
	Keys     []friendlyKeys
}

type friendlyLink struct {
	ID           string
	From         string
	To           string
	Desc         string
	AssignedTo   string
	AssignedToID wasabi.GoogleID
	ThrowOrder   float64
	Distance     float64
}

type friendlyMarker struct {
	ID           string
	Portal       string
	Type         wasabi.MarkerType
	Comment      string
	AssignedTo   string
	AssignedToID wasabi.GoogleID
}

type friendlyKeys struct {
	ID       wasabi.PortalID
	Portal   string
	Required int
	OnHand   int
}

// takes a populated op and returns a friendly named version
func pDrawFriendlyNames(op *wasabi.Operation) (pdrawFriendly, error) {
	var err error
	var friendly pdrawFriendly
	friendly.ID = op.ID
	friendly.TeamID = op.TeamID
	friendly.Name = op.Name
	friendly.Color = op.Color
	friendly.Modified = op.Modified

	friendly.Agent, err = op.Gid.IngressName()
	if err != nil {
		return friendly, err
	}
	friendly.Team, err = op.TeamID.Name()
	if err != nil {
		return friendly, err
	}

	var portals = make(map[wasabi.PortalID]wasabi.Portal)

	for _, p := range op.OpPortals {
		portals[p.ID] = p
	}

	for _, l := range op.Links {
		var fl friendlyLink
		fl.ID = l.ID
		fl.Desc = l.Desc
		fl.ThrowOrder = l.ThrowOrder
		fl.From = portals[l.From].Name
		fl.To = portals[l.To].Name
		fl.AssignedTo, _ = l.AssignedTo.IngressName()
		fl.AssignedToID = l.AssignedTo
		fl.Distance = lldistance(portals[l.From].Lat, portals[l.From].Lon, portals[l.To].Lat, portals[l.To].Lon)
		friendly.Links = append(friendly.Links, fl)
	}

	for _, m := range op.Markers {
		var fm friendlyMarker
		fm.ID = m.ID
		fm.Type = m.Type
		fm.Comment = m.Comment
		fm.Portal = portals[m.PortalID].Name
		fm.AssignedTo, _ = m.AssignedTo.IngressName()
		friendly.Markers = append(friendly.Markers, fm)
	}

	var keys = make(map[wasabi.PortalID]friendlyKeys)
	for _, l := range op.Links {
		_, ok := keys[l.To]
		if !ok {
			keys[l.To] = friendlyKeys{
				ID:       l.To,
				Portal:   portals[l.To].Name,
				Required: 1,
				OnHand:   0,
			}
		} else {
			tmp := keys[l.To]
			tmp.Required++
			keys[l.To] = tmp
		}
	}
	for _, f := range keys {
		friendly.Keys = append(friendly.Keys, f)
	}

	return friendly, nil
}

func lldistance(startLat, startLon, endLat, endLon string) float64 {
	sl, _ := strconv.ParseFloat(startLat, 64)
	startrl := math.Pi * sl / 180.0
	el, _ := strconv.ParseFloat(endLat, 64)
	endrl := math.Pi * el / 180.0

	t, _ := strconv.ParseFloat(startLon, 64)
	th, _ := strconv.ParseFloat(endLon, 64)
	rt := math.Pi * (t - th) / 180.0

	dist := math.Sin(startrl)*math.Sin(endrl) + math.Cos(startrl)*math.Cos(endrl)*math.Cos(rt)
	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / math.Pi
	dist = dist * 60 * 1.1515 * 1.609344
	return math.RoundToEven(dist)
}
