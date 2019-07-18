package wasabee

import (
	"database/sql"
	"errors"
	"fmt"
)

// this is a kludge and needs to go away
func pdrawAuthorized(gid GoogleID, oid OperationID) (bool, TeamID, error) {
	var opgid GoogleID
	var teamID TeamID
	var authorized bool
	err := db.QueryRow("SELECT gid, teamID FROM operation WHERE ID = ?", oid).Scan(&opgid, &teamID)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return false, "", err
	}
	if err != nil && err == sql.ErrNoRows {
		authorized = true
	}
	if opgid == gid {
		authorized = true
	}
	if !authorized {
		return false, teamID, errors.New("unauthorized: this operation owned by someone else")
	}
	return authorized, teamID, nil
}

// GetTeamID returns the teamID for an op
func (opID OperationID) GetTeamID() (TeamID, error) {
	var teamID TeamID
	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", opID).Scan(&teamID)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	if err != nil && err == sql.ErrNoRows {
		return "", nil
	}
	return teamID, nil
}

func (opID OperationID) ReadAccess(gid GoogleID) bool {
	var teamID TeamID
	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", opID).Scan(&teamID)
	if err != nil {
		Log.Error(err)
		return false
	}
	inteam, err := gid.AgentInTeam(teamID, false)
	if err != nil {
		Log.Error(err)
		return false
	}
	return inteam
}

// WriteAccess determines if an agent has write access to an op
func (opID OperationID) WriteAccess(gid GoogleID) bool {
	return opID.IsOwner(gid)
}

// IsOwner returns a bool value determining if the operation is owned by the specified googleID
func (opID OperationID) IsOwner(gid GoogleID) bool {
	var c int
	err := db.QueryRow("SELECT COUNT(*) FROM operation WHERE ID = ? and gid = ?", opID, gid).Scan(&c)
	if err != nil {
		Log.Error(err)
		return false
	}
	if c < 1 {
		return false
	}
	return true
}

// Chown changes an operation's owner
func (opID OperationID) Chown(gid GoogleID, to string) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, opID)
		Log.Error(err)
		return err
	}

	togid, err := ToGid(to)
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE operation SET gid = ? WHERE ID = ?", togid, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

// Chgrp changes an operation's team -- because UNIX libc function names are cool, yo
func (opID OperationID) Chgrp(gid GoogleID, to TeamID) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("%s not current owner of op %s", gid, opID)
		Log.Error(err)
		return err
	}

	// check to see if the team really exists
	if _, err := to.Name(); err != nil {
		Log.Error(err)
		return err
	}

	_, err := db.Exec("UPDATE operation SET teamID = ? WHERE ID = ?", to, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}