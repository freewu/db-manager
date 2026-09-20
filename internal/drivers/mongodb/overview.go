package mongodb

import (
	"context"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/format"
	"dbmanager/internal/models"
)

// Overview implements drivers.Overviewer: the MongoDB status page.
//
// Like the other engines this answers a fixed set of questions in one pass:
// what the server is, how loaded it is, what it is doing right now and how much
// data it holds. Everything comes from serverStatus plus one dbStats per
// database; the session fields (id, name, read-only, ...) are the service's.
func (c *Conn) Overview(ctx context.Context) (*models.ServerOverview, error) {
	var status serverStatus
	if err := c.adminCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}, &status); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read server status")
	}

	warnings := []string{}
	groups := []models.OverviewGroup{
		serverGroup(status),
		connectionGroup(status),
		operationGroup(status),
		memoryGroup(status),
	}

	if cache, ok := cacheGroup(status); ok {
		groups = append(groups, cache)
	}

	replica, err := c.replicaGroup(ctx)
	switch {
	case err == nil && replica != nil:
		groups = append(groups, *replica)
	case err != nil:
		// A standalone server has nothing to say here; anything else is worth
		// reporting because a broken replica set is a real problem.
		warnings = append(warnings, "Replication state could not be read: "+err.Error())
	}

	databases, dbWarnings := c.databaseTable(ctx)
	warnings = append(warnings, dbWarnings...)

	return &models.ServerOverview{
		ServerVersion: status.Version,
		Supported:     true,
		Warnings:      warnings,
		Mongo: &models.MongoOverview{
			Groups:    groups,
			Databases: databases,
		},
	}, nil
}

// --- server status document -------------------------------------------------

// serverStatus is the trimmed serverStatus reply. Only the fields the page
// shows are declared; everything else the server sends is ignored, and a field
// an older or newer server does not send stays zero.
type serverStatus struct {
	Host        string    `bson:"host"`
	Version     string    `bson:"version"`
	Process     string    `bson:"process"`
	Uptime      int64     `bson:"uptime"`
	LocalTime   time.Time `bson:"localTime"`
	Connections struct {
		Current      int64 `bson:"current"`
		Available    int64 `bson:"available"`
		TotalCreated int64 `bson:"totalCreated"`
		Active       int64 `bson:"active"`
	} `bson:"connections"`
	Opcounters struct {
		Insert  int64 `bson:"insert"`
		Query   int64 `bson:"query"`
		Update  int64 `bson:"update"`
		Delete  int64 `bson:"delete"`
		GetMore int64 `bson:"getmore"`
		Command int64 `bson:"command"`
	} `bson:"opcounters"`
	Mem struct {
		Resident int64 `bson:"resident"`
		Virtual  int64 `bson:"virtual"`
		Mapped   int64 `bson:"mapped"`
	} `bson:"mem"`
	Metrics struct {
		Document struct {
			Deleted  int64 `bson:"deleted"`
			Inserted int64 `bson:"inserted"`
			Returned int64 `bson:"returned"`
			Updated  int64 `bson:"updated"`
		} `bson:"document"`
		QueryExecutor struct {
			Scanned        int64 `bson:"scanned"`
			ScannedObjects int64 `bson:"scannedObjects"`
		} `bson:"queryExecutor"`
	} `bson:"metrics"`
	StorageEngine struct {
		Name string `bson:"name"`
	} `bson:"storageEngine"`
	ExtraInfo struct {
		PageFaults int64 `bson:"page_faults"`
	} `bson:"extra_info"`
	WiredTiger struct {
		Cache struct {
			BytesInCache    int64 `bson:"bytes currently in the cache"`
			MaxBytes        int64 `bson:"maximum bytes configured"`
			DirtyBytes      int64 `bson:"tracked dirty bytes in the cache"`
			BytesRead       int64 `bson:"bytes read into cache"`
			BytesWritten    int64 `bson:"bytes written from cache"`
			PagesRead       int64 `bson:"pages read into cache"`
			PagesRequested  int64 `bson:"pages requested from the cache"`
			PagesWritten    int64 `bson:"pages written from cache"`
			UnmodifiedEvict int64 `bson:"unmodified pages evicted"`
			ModifiedEvict   int64 `bson:"modified pages evicted"`
		} `bson:"cache"`
	} `bson:"wiredTiger"`
}

func serverGroup(s serverStatus) models.OverviewGroup {
	metrics := []models.OverviewMetric{
		format.Metric("Version", format.Dash(s.Version), "serverStatus.version — the storage server's own version"),
		format.Metric("Host", format.Dash(s.Host), "serverStatus.host"),
		format.Metric("Process", format.Dash(s.Process), "serverStatus.process: mongod or mongos"),
		format.Metric("Uptime", format.FormatDuration(float64(s.Uptime)), "serverStatus.uptime"),
	}
	if s.StorageEngine.Name != "" {
		metrics = append(metrics, format.Metric("Storage engine", s.StorageEngine.Name, "serverStatus.storageEngine.name"))
	}
	if !s.LocalTime.IsZero() {
		metrics = append(metrics, format.Metric("Server clock", formatTime(s.LocalTime),
			"serverStatus.localTime — compare with your own clock to spot drift"))
	}
	return models.OverviewGroup{Title: "Server", Metrics: metrics}
}

func connectionGroup(s serverStatus) models.OverviewGroup {
	used := float64(s.Connections.Current)
	total := used + float64(s.Connections.Available)
	usage := format.Metric("Usage", format.FormatPercent(used, total),
		"connections.current / (current + available)")
	switch {
	case total > 0 && used/total >= 0.9:
		usage.State = "bad"
	case total > 0 && used/total >= 0.8:
		usage.State = "warn"
	}

	metrics := []models.OverviewMetric{
		format.Metric("Current", format.FormatCount(s.Connections.Current), "serverStatus.connections.current"),
		format.Metric("Available", format.FormatCount(s.Connections.Available), "serverStatus.connections.available"),
		format.Metric("Total created", format.FormatCount(s.Connections.TotalCreated), "serverStatus.connections.totalCreated"),
		usage,
	}
	if s.Connections.Active > 0 {
		metrics = append(metrics, format.Metric("Active", format.FormatCount(s.Connections.Active),
			"serverStatus.connections.active — connections currently running an operation"))
	}
	return models.OverviewGroup{
		Title:   "Connections",
		Note:    "MongoDB reports counts rather than a process list.",
		Metrics: metrics,
	}
}

func operationGroup(s serverStatus) models.OverviewGroup {
	metrics := []models.OverviewMetric{
		format.Metric("Insert", format.FormatCount(s.Opcounters.Insert), "serverStatus.opcounters.insert — since startup"),
		format.Metric("Query", format.FormatCount(s.Opcounters.Query), "serverStatus.opcounters.query"),
		format.Metric("Update", format.FormatCount(s.Opcounters.Update), "serverStatus.opcounters.update"),
		format.Metric("Delete", format.FormatCount(s.Opcounters.Delete), "serverStatus.opcounters.delete"),
		format.Metric("Command", format.FormatCount(s.Opcounters.Command), "serverStatus.opcounters.command"),
		format.Metric("Get more", format.FormatCount(s.Opcounters.GetMore), "serverStatus.opcounters.getmore"),
		format.Metric("Documents returned", format.FormatCount(s.Metrics.Document.Returned), "serverStatus.metrics.document.returned"),
		format.Metric("Documents scanned", format.FormatCount(s.Metrics.QueryExecutor.ScannedObjects),
			"serverStatus.metrics.queryExecutor.scannedObjects — much larger than returned means missing indexes"),
	}
	return models.OverviewGroup{Title: "Operations", Metrics: metrics}
}

func memoryGroup(s serverStatus) models.OverviewGroup {
	metrics := []models.OverviewMetric{
		format.Metric("Resident", format.FormatBytes(s.Mem.Resident*1024*1024), "serverStatus.mem.resident (MiB)"),
		format.Metric("Virtual", format.FormatBytes(s.Mem.Virtual*1024*1024), "serverStatus.mem.virtual (MiB)"),
		format.Metric("Page faults", format.FormatCount(s.ExtraInfo.PageFaults), "serverStatus.extra_info.page_faults"),
	}
	if s.Mem.Mapped > 0 {
		metrics = append(metrics, format.Metric("Mapped", format.FormatBytes(s.Mem.Mapped*1024*1024), "serverStatus.mem.mapped (MiB)"))
	}
	return models.OverviewGroup{Title: "Memory", Metrics: metrics}
}

// cacheGroup reports the WiredTiger cache, which is where a MongoDB server
// spends its memory. It is absent on engines (or builds) without WiredTiger.
func cacheGroup(s serverStatus) (models.OverviewGroup, bool) {
	cache := s.WiredTiger.Cache
	if cache.MaxBytes == 0 {
		return models.OverviewGroup{}, false
	}
	metrics := []models.OverviewMetric{
		format.Metric("In cache", format.FormatBytes(cache.BytesInCache), "wiredTiger.cache.bytes currently in the cache"),
		format.Metric("Configured", format.FormatBytes(cache.MaxBytes), "wiredTiger.cache.maximum bytes configured"),
		format.Metric("Cache used", format.FormatPercent(float64(cache.BytesInCache), float64(cache.MaxBytes)),
			"bytes in cache / configured — a full cache is normal, an eviction storm is not"),
		format.Metric("Dirty", format.FormatBytes(cache.DirtyBytes),
			"wiredTiger.cache.tracked dirty bytes in the cache — the part still to be written"),
		format.Metric("Read into cache", format.FormatBytes(cache.BytesRead), "wiredTiger.cache.bytes read into cache"),
		format.Metric("Written from cache", format.FormatBytes(cache.BytesWritten), "wiredTiger.cache.bytes written from cache"),
		format.Metric("Pages evicted", format.FormatCount(cache.ModifiedEvict+cache.UnmodifiedEvict),
			"wiredTiger.cache pages evicted (dirty and clean)"),
	}
	if cache.PagesRequested > 0 {
		hit := 1 - float64(cache.PagesRead)/float64(cache.PagesRequested)
		metrics = append(metrics, format.Metric("Cache hit rate", format.FormatPercent(hit, 1),
			"1 − pages read into cache / pages requested from the cache"))
	}
	return models.OverviewGroup{Title: "WiredTiger cache", Metrics: metrics}, true
}

// helloReply is the subset of the hello/isMaster reply that identifies a
// replica set member.
type helloReply struct {
	SetName           string   `bson:"setName"`
	SetVersion        int64    `bson:"setVersion"`
	Hosts             []string `bson:"hosts"`
	Primary           string   `bson:"primary"`
	Me                string   `bson:"me"`
	IsWritablePrimary bool     `bson:"isWritablePrimary"`
	Secondary         bool     `bson:"secondary"`
	ArbiterOnly       bool     `bson:"arbiterOnly"`
}

// replicaGroup describes a replica set deployment. It returns nil (and no
// error) for a standalone server.
func (c *Conn) replicaGroup(ctx context.Context) (*models.OverviewGroup, error) {
	var reply helloReply
	if err := c.adminCommand(ctx, bson.D{{Key: "hello", Value: 1}}, &reply); err != nil {
		return nil, err
	}
	if reply.SetName == "" {
		return nil, nil
	}

	role := "secondary"
	switch {
	case reply.ArbiterOnly:
		role = "arbiter"
	case reply.IsWritablePrimary:
		role = "primary"
	case reply.Secondary:
		role = "secondary"
	}
	metrics := []models.OverviewMetric{
		format.Metric("Replica set", reply.SetName, "hello.setName"),
		format.Metric("Members", format.FormatCount(int64(len(reply.Hosts))), "hello.hosts"),
		format.Metric("Primary", format.Dash(reply.Primary), "hello.primary"),
		format.Metric("This node", format.Dash(reply.Me), "hello.me"),
		format.Metric("Role", role, "hello.isWritablePrimary / secondary / arbiterOnly"),
	}
	sort.Strings(reply.Hosts)
	return &models.OverviewGroup{
		Title:   "Replica set",
		Note:    strings.Join(reply.Hosts, ", "),
		Metrics: metrics,
	}, nil
}

// --- per database table -----------------------------------------------------

// dbStats is the trimmed dbStats reply.
type dbStats struct {
	DB          string `bson:"db"`
	Collections int64  `bson:"collections"`
	Views       int64  `bson:"views"`
	Objects     int64  `bson:"objects"`
	DataSize    int64  `bson:"dataSize"`
	StorageSize int64  `bson:"storageSize"`
	Indexes     int64  `bson:"indexes"`
	IndexSize   int64  `bson:"indexSize"`
}

// databaseTable reads the size of every database.
//
// A database the user cannot read from is skipped with a warning instead of
// failing the page: an overview that shows nine databases and says why the
// tenth is missing beats an error page.
func (c *Conn) databaseTable(ctx context.Context) (*models.OverviewTable, []string) {
	names, err := c.Databases(ctx)
	if err != nil {
		return nil, []string{"Databases could not be listed: " + err.Error()}
	}

	stats := make([]dbStats, 0, len(names))
	warnings := []string{}
	for _, name := range names {
		var stat dbStats
		if err := c.client.Database(name).RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&stat); err != nil {
			warnings = append(warnings, "dbStats failed for "+name)
			continue
		}
		if stat.DB == "" {
			stat.DB = name
		}
		stats = append(stats, stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].StorageSize != stats[j].StorageSize {
			return stats[i].StorageSize > stats[j].StorageSize
		}
		return stats[i].DB < stats[j].DB
	})

	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.DB,
			format.FormatCount(stat.Collections),
			format.FormatCount(stat.Objects),
			format.FormatBytes(stat.DataSize),
			format.FormatBytes(stat.StorageSize),
			format.FormatCount(stat.Indexes),
			format.FormatBytes(stat.IndexSize),
		})
	}
	return &models.OverviewTable{
		Title:   "Databases",
		Columns: []string{"Database", "Collections", "Documents", "Data", "Storage", "Indexes", "Index size"},
		Rows:    rows,
		Note:    "dbStats per database; documents and sizes are the server's own estimates.",
	}, warnings
}

// adminCommand runs a command against the admin database, which is where the
// server-wide commands live.
func (c *Conn) adminCommand(ctx context.Context, cmd bson.D, out any) error {
	return c.client.Database(defaultDatabase).RunCommand(ctx, cmd).Decode(out)
}
