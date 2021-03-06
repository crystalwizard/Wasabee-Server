package wasabee

import (
	"database/sql"
	"fmt"

	// need a comment here to make lint happy
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Connect tries to establish a connection to a MySQL/MariaDB database under the given URI and initializes the tables if they don"t exist yet.
func Connect(uri string) error {
	Log.Debugf("Connecting to database at %s", uri)
	result, err := sql.Open("mysql", uri)
	if err != nil {
		return err
	}
	db = result

	// Print database version
	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return err
	}
	Log.Infof("Database version: %s", version)

	setupTables()
	return nil
}

// Disconnect closes the database connection
// called only at server shutdown
func Disconnect() {
	Log.Debug("Disconnecting from database")
	if err := db.Close(); err != nil {
		Log.Error(err)
	}
}

// setupTables checks for the existence of tables and creates them if needed
func setupTables() {
	var t = []struct {
		tablename string
		creation  string
	}{
		// agent must come first, team must come second, operation must come third, the rest can be in alphabetical order
		{"agent", `CREATE TABLE agent ( gid varchar(32) NOT NULL, iname varchar(64) DEFAULT NULL, level tinyint(4) NOT NULL DEFAULT '1', lockey varchar(64) DEFAULT NULL, VVerified tinyint(1) NOT NULL DEFAULT '0', Vblacklisted tinyint(1) NOT NULL DEFAULT '0', Vid varchar(40) DEFAULT NULL, RocksVerified tinyint(1) NOT NULL DEFAULT '0', RAID tinyint(1) NOT NULL DEFAULT '0', RISC tinyint(1) NOT NULL DEFAULT '0', PRIMARY KEY (gid), UNIQUE KEY iname (iname), UNIQUE KEY lockey (lockey), UNIQUE KEY Vid (Vid)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"team", `CREATE TABLE team ( teamID varchar(64) NOT NULL, owner varchar(32) NOT NULL, name varchar(64) DEFAULT NULL, rockskey varchar(32) DEFAULT NULL, rockscomm varchar(32) DEFAULT NULL, PRIMARY KEY (teamID), KEY fk_team_owner (owner), CONSTRAINT fk_team_owner FOREIGN KEY (owner) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"operation", `CREATE TABLE operation ( ID varchar(64) NOT NULL, name varchar(128) NOT NULL DEFAULT 'new op', gid varchar(32) NOT NULL, color varchar(16) NOT NULL DEFAULT 'groupa', teamID varchar(64) NOT NULL DEFAULT '', modified datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, comment text, PRIMARY KEY (ID), KEY gid (gid), KEY teamID (teamID), CONSTRAINT fk_operation_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},

		{"agentextras", `CREATE TABLE agentextras ( gid varchar(32) NOT NULL, picurl text, UNIQUE KEY gid (gid), CONSTRAINT fk_extra_agent FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"agentteams", `CREATE TABLE agentteams ( teamID varchar(64) NOT NULL, gid varchar(32) NOT NULL, state enum('Off','On') NOT NULL DEFAULT 'Off', color varchar(32) NOT NULL DEFAULT 'boots', displayname varchar(32) DEFAULT NULL,  PRIMARY KEY (teamID,gid), KEY GIDKEY (gid), CONSTRAINT fk_agent_teams FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE, CONSTRAINT fk_t_teams FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"anchor", `CREATE TABLE anchor ( opID varchar(64) DEFAULT NULL, portalID varchar(64) DEFAULT NULL, PRIMARY KEY anchor (opID,portalID), CONSTRAINT fk_operation_id_anchor FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"firebase", `CREATE TABLE firebase ( gid varchar(32) NOT NULL, token varchar(4092) NOT NULL, KEY fk_gid (gid), CONSTRAINT fk_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"link", `CREATE TABLE link ( ID varchar(64) NOT NULL, fromPortalID varchar(64) NOT NULL, toPortalID varchar(64) NOT NULL, opID varchar(64) NOT NULL, description text, gid varchar(32) DEFAULT NULL, throworder int(11) DEFAULT '0', completed tinyint(1) NOT NULL DEFAULT '0', color varchar(16) NOT NULL DEFAULT 'main', PRIMARY KEY (ID,opID), KEY fk_operation_id_link (opID), KEY fk_link_gid (gid), CONSTRAINT fk_link_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE SET NULL, CONSTRAINT fk_operation_id_link FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"locations", `CREATE TABLE locations ( gid varchar(32) NOT NULL, upTime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, loc point NOT NULL, PRIMARY KEY (gid), SPATIAL KEY sp (loc)) ENGINE=Aria DEFAULT CHARSET=utf8mb4 PAGE_CHECKSUM=1;`},
		{"marker", `CREATE TABLE marker ( ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, type varchar(128) NOT NULL, gid varchar(32) DEFAULT NULL, comment text, complete tinyint(1) NOT NULL DEFAULT '0', state enum('pending','assigned','acknowledged','completed') NOT NULL DEFAULT 'pending', completedBy varchar(32) DEFAULT NULL, oporder int NOT NULL DEFAULT 0, PRIMARY KEY (ID,opID), KEY fk_operation_marker (opID), KEY fk_marker_gid (gid), CONSTRAINT fk_marker_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE SET NULL, CONSTRAINT fk_operation_marker FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"messagelog", `CREATE TABLE messagelog ( timestamp datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, gid varchar(32) NOT NULL, message text NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"opkeys", `CREATE TABLE opkeys ( opID varchar(64) NOT NULL, portalID varchar(64) NOT NULL, gid varchar(32) NOT NULL, onhand int(11) NOT NULL DEFAULT '0', UNIQUE KEY key_unique (opID,portalID,gid), KEY fk_operation_id_keys (opID), CONSTRAINT fk_operation_id_keys FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"portal", `CREATE TABLE portal ( ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, name varchar(128) NOT NULL, loc point NOT NULL, comment text, hardness varchar(64) DEFAULT NULL, PRIMARY KEY ID (ID,opID), KEY fk_operation_id (opID), SPATIAL KEY sp_portal (loc)) ENGINE=Aria DEFAULT CHARSET=utf8mb4 PAGE_CHECKSUM=1;`},
		{"telegram", `CREATE TABLE telegram ( telegramID bigint(20) NOT NULL, telegramName varchar(32) NOT NULL, gid varchar(32) NOT NULL, verified tinyint(1) NOT NULL DEFAULT '0', authtoken varchar(32) DEFAULT NULL, PRIMARY KEY (telegramID), UNIQUE KEY gid (gid), CONSTRAINT fk_agent_telegram FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		{"opteams", `CREATE TABLE opteams (teamID varchar(64) NOT NULL, opID varchar(64) NOT NULL, permission enum('read','write','assignedonly') NOT NULL DEFAULT 'read', KEY opID (opID), KEY teamID (teamID), CONSTRAINT fk_ops_teamID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE, CONSTRAINT fk_teamIDs_op FOREIGN KEY (teamID) REFERENCES team (teamID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`},

		// reg_form must come before reg_entry
		// {"reg_form", `CREATE TABLE reg_form ( ID varchar(64) NOT NULL, opID varchar(64) NOT NULL, date datetime DEFAULT NULL, desc text, PRIMARY KEY (ID), UNIQUE KEY opID (opID), CONSTRAINT fk_opID FOREIGN KEY (opID) REFERENCES operation (ID) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
		// {"reg_entry", `CREATE TABLE reg_entry ( gid varchar(32) NOT NULL, formID varchar(64) NOT NULL, homecell varchar(32) NOT NULL, radius int(32) NOT NULL DEFAULT '100', roles set('keyfarm','boots','fielding','keytransport','other') DEFAULT NULL, special set('booster','bgan','onyx','hike') DEFAULT NULL, PRIMARY KEY (gid), UNIQUE KEY gid (gid,formID), KEY fk_reg_formID (formID), CONSTRAINT fk_reg_formID FOREIGN KEY (formID) REFERENCES reg_form (ID) ON DELETE CASCADE, CONSTRAINT fk_reg_gid FOREIGN KEY (gid) REFERENCES agent (gid) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`},
	}

	var table string
	// use a tranaction to AVOID concurrency in this logic
	// it is possible for these to go in out-of-order and fk problems to show up under rare circumstances
	tx, err := db.Begin()
	if err != nil {
		Log.Error(err)
		panic(err)
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=0")
	if err != nil {
		Log.Error(err)
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			Log.Critical(err)
		}
		// tx is complete, use db
		_, err = db.Exec("SET FOREIGN_KEY_CHECKS=1")
		if err != nil {
			Log.Critical(err)
		}
	}()
	for _, v := range t {
		q := fmt.Sprintf("SHOW TABLES LIKE '%s'", v.tablename)
		err = tx.QueryRow(q).Scan(&table)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			continue
		}
		if err == sql.ErrNoRows || table == "" {
			Log.Noticef("Setting up '%s' table...", v.tablename)
			_, err = tx.Exec(v.creation)
			if err != nil {
				Log.Critical(err)
			}
		}
	}
	_, err = tx.Exec("SET FOREIGN_KEY_CHECKS=1")
	if err != nil {
		Log.Critical(err)
	}
	err = tx.Commit() // the defer'd rollback will not have anything to rollback...
	if err != nil {
		Log.Critical(err)
	}
	// defer'd func runs here
}

// MakeNullString is used for values that may & might be inserted/updated as NULL in the database
func MakeNullString(in interface{}) sql.NullString {
	var s string

	tmp, ok := in.(string)
	if ok {
		s = tmp
	} else {
		tmp, ok := in.(fmt.Stringer)
		if !ok {
			return sql.NullString{}
		}
		s = tmp.String()
	}
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}
