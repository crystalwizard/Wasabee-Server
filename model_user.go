package PhDevBin

import (
	"database/sql"
	"errors"
	"fmt"
)

// GoogleID is the primary location for interfacing with the user type
type GoogleID string

// TeamID is the primary means for interfacing with teams
type TeamID string

// LocKey is the location share key
type LocKey string

// EnlID is a V EnlID
type EnlID string

// TelegramID is a Telegram user ID
// type TelegramID int

// UserData is the complete user struct, used for the /me page
type UserData struct {
	GoogleID      GoogleID
	IngressName   string
	Level         float64
	LocationKey   string
	OwnTracksPW   string
	VVerified     bool
	VBlacklisted  bool
	Vid           EnlID
	OwnTracksJSON string
	Teams         []struct {
		ID    string
		Name  string
		State string
	}
	OwnedTeams []struct {
		ID   string
		Name string
	}
	Telegram struct {
		UserName  string
		ID        int // json changes this to float64, should we just leave it float64 the whole way?
		Verified  bool
		Authtoken string `json:"at,omitempty"`
	}
	Home struct {
		Lat    float64
		Lon    float64
		Radius float64
	} // unused currently Tony wants it, requires schema change
}

// InitUser is called from Oauth callback to set up a user for the first time.
// It also checks and updates V data. It returns true if the user is authorized to continue, false if the user is blacklisted or otherwise locked at V
func (gid GoogleID) InitUser() (bool, error) {
	var vdata Vresult

	err := gid.VSearch(&vdata)
	if err != nil {
		Log.Notice(err)
	}

	var tmpName string
	if vdata.Data.Agent != "" {
		tmpName = vdata.Data.Agent
		err = gid.VUpdate(&vdata)
		if err != nil {
			Log.Notice(err)
			return false, err
		}
		if vdata.Data.Blacklisted == true {
			err = errors.New("Blacklisted at V")
			return false, err
		}
		if vdata.Data.Flagged == true {
			err = errors.New("Flagged at V")
			return false, err
		}
		if vdata.Data.Banned == true {
			err = errors.New("Banned at V")
			return false, err
		}
		if vdata.Data.Quarantine == true {
			err = errors.New("Quarantined at V")
			return false, err
		}
	} else {
		tmpName = "Agent_" + gid.String()[:8]
	}

	// this block should be skipped if the user is already in the database, using IGNORE is just lazy...
	lockey, err := GenerateSafeName()
	if err != nil {
		Log.Notice(err)
		return false, err
	}
	_, err = db.Exec("INSERT IGNORE INTO user (gid, iname, level, lockey, OTpassword, VVerified, VBlacklisted, Vid) VALUES (?,?,?,?,NULL,?,?,?)",
		gid, tmpName, vdata.Data.Level, lockey, vdata.Data.Verified, vdata.Data.Blacklisted, vdata.Data.EnlID)
	if err != nil {
		Log.Notice(err)
		return false, err
	}
	_, err = db.Exec("INSERT IGNORE INTO locations VALUES (?,NOW(),POINT(0,0))", gid)
	if err != nil {
		Log.Notice(err)
		return false, err
	}

	_, err = db.Exec("INSERT IGNORE INTO otdata VALUES (?,'{ }')", gid)
	if err != nil {
		Log.Notice(err)
		return false, err
	}

	return true, nil
}

// SetIngressName is called to update the agent's ingress name in the database
// The ingress name cannot be updated if V verification has taken place
func (gid GoogleID) SetIngressName(name string) error {
	// if VVerified, ignore name changes -- let the V functions take care of that
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ? AND VVerified = 0", name, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// SetOwnTracksPW updates the database with a new OwnTracks password for a given user
// TODO: move to model_owntracks.go
func (gid GoogleID) SetOwnTracksPW(otpw string) error {
	_, err := db.Exec("UPDATE user SET OTpassword = PASSWORD(?) WHERE gid = ?", otpw, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// VerifyOwnTracksPW is used to check that the supplied password matches the stored password hash for the given user
// upon success it returns the gid for the lockey (which is also the owntracks username), on failure it returns ""
// TODO: move to model_owntracks.go
func (lockey LocKey) VerifyOwnTracksPW(otpw string) (GoogleID, error) {
	var gid GoogleID

	r := db.QueryRow("SELECT gid FROM user WHERE OTpassword = PASSWORD(?) AND lockey = ?", otpw, lockey)
	err := r.Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	if err != nil && err == sql.ErrNoRows {
		return "", nil
	}

	return gid, nil
}

// RemoveFromTeam updates the team list to remove the user.
// XXX move to model_team.go
func (gid GoogleID) RemoveFromTeam(team TeamID) error {
	if _, err := db.Exec("DELETE FROM userteams WHERE gid = ? AND teamID = ?", team, gid); err != nil {
		Log.Notice(err)
	}
	return nil
}

// SetTeamState updates the users state on the team (Off|On|Primary)
// XXX move to model_team.go
func (gid GoogleID) SetTeamState(teamID TeamID, state string) error {
	if state == "Primary" {
		_ = gid.ClearPrimaryTeam()
	}

	if _, err := db.Exec("UPDATE userteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		Log.Notice(err)
	}
	return nil
}

// SetTeamStateName -- same as SetTeamState, but takes a team's human name rather than ID
// BUG: if multiple teams use the same name this will not work
// XXX move to model_team.go
func (gid GoogleID) SetTeamStateName(teamname string, state string) error {
	Log.Debug(teamname)
	var id TeamID
	row := db.QueryRow("SELECT teamID FROM teams WHERE name = ?", teamname)
	err := row.Scan(&id)
	if err != nil {
		Log.Notice(err)
	}

	return gid.SetTeamState(id, state)
}

// Gid converts a location share key to a user's gid
// TODO: quit using a prebuilt query from database.go
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	r := lockeyToGid.QueryRow(lockey)
	err := r.Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}

	return gid, nil
}

// GetUserData populates a UserData struct based on the gid
func (gid GoogleID) GetUserData(ud *UserData) error {
	var ot sql.NullString

	ud.GoogleID = gid

	row := db.QueryRow("SELECT iname, level, lockey, OTpassword, VVerified, VBlacklisted, Vid FROM user WHERE gid = ?", gid)
	err := row.Scan(&ud.IngressName, &ud.Level, &ud.LocationKey, &ot, &ud.VVerified, &ud.VBlacklisted, &ud.Vid)
	if err != nil {
		Log.Notice(err)
		return err
	}

	if ot.Valid {
		ud.OwnTracksPW = ot.String
	}

	var teamname sql.NullString
	var tmp struct {
		ID    string
		Name  string
		State string
	}

	rows, err := db.Query("SELECT t.teamID, t.name, x.state "+
		"FROM teams=t, userteams=x "+
		"WHERE x.gid = ? AND x.teamID = t.teamID", gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmp.ID, &teamname, &tmp.State)
		if err != nil {
			Log.Error(err)
			return err
		}
		// teamname can be null
		if teamname.Valid {
			tmp.Name = teamname.String
		} else {
			tmp.Name = ""
		}
		ud.Teams = append(ud.Teams, tmp)
	}

	var ownedTeam struct {
		ID   string
		Name string
	}
	ownedTeamRow, err := db.Query("SELECT teamID, name FROM teams WHERE owner = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer ownedTeamRow.Close()
	for ownedTeamRow.Next() {
		err := ownedTeamRow.Scan(&ownedTeam.ID, &teamname)
		if err != nil {
			Log.Error(err)
			return err
		}
		// can be null -- but this should be changed
		if teamname.Valid {
			ownedTeam.Name = teamname.String
		} else {
			ownedTeam.Name = ""
		}
		ud.OwnedTeams = append(ud.OwnedTeams, ownedTeam)
	}

	// XXX cannot be null -- just JOIN in main query
	var otJSON sql.NullString
	rows2 := db.QueryRow("SELECT otdata FROM otdata WHERE gid = ?", gid)
	err = rows2.Scan(&otJSON)
	if err != nil && err.Error() == "sql: no rows in result set" {
		ud.OwnTracksJSON = "{ }"
		return nil
	}
	if err != nil {
		Log.Error(err)
		return err
	}
	if otJSON.Valid {
		ud.OwnTracksJSON = otJSON.String
	} else {
		ud.OwnTracksJSON = "{ }"
	}
	// defer rows2.Close()

	var authtoken sql.NullString
	rows3 := db.QueryRow("SELECT telegramName, telegramID, verified, authtoken FROM telegram WHERE gid = ?", gid)
	err = rows3.Scan(&ud.Telegram.UserName, &ud.Telegram.ID, &ud.Telegram.Verified, &authtoken)
	if err != nil && err.Error() == "sql: no rows in result set" {
		ud.Telegram.ID = 0
		ud.Telegram.Verified = false
		ud.Telegram.Authtoken = ""
	} else if err != nil {
		Log.Error(err)
		return err
	}
	ud.Telegram.Authtoken = authtoken.String
	// defer rows3.Close()

	return nil
}

// UserLocation updates the database to reflect a user's current location
// OwnTracks data is updated to reflect the change
// TODO: react based on the location
func (gid GoogleID) UserLocation(lat, lon, source string) error {
	var point string

	// sanity checing on bounds?
	// YES, store lon,lat -- the ST_ functions expect it this way
	point = fmt.Sprintf("POINT(%s %s)", lon, lat)
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = NOW() WHERE gid = ?", point, gid); err != nil {
		Log.Notice(err)
		return err
	}

	if source != "OwnTracks" {
		err := gid.ownTracksExternalUpdate(lat, lon)
		if err != nil {
			Log.Notice(err)
			return err
		}
		// put it out onto MQTT
	}

	// XXX check for targets in range

	// XXX send notifications

	return nil
}

func (gid GoogleID) String() string {
	return string(gid)
}

func (eid EnlID) String() string {
	return string(eid)
}
