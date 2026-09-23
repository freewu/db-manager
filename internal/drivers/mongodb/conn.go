package mongodb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Paging and timeout defaults. They mirror sqlbase so the UI behaves the same
// whichever engine is behind it.
const (
	defaultPageSize  = 200
	maxPageSize      = 5000
	defaultMaxRows   = 1000
	maxMaxRows       = 50000
	defaultTimeoutMS = 60_000

	// fieldSampleSize is how many documents Structure reads to infer fields.
	fieldSampleSize = 100
	// maxStatsCollections caps the per-collection storage statistics Objects
	// collects. Past that the listing stays cheap (one listCollections call)
	// and the size columns stay empty rather than turning a tree expansion
	// into hundreds of commands.
	maxStatsCollections = 64
	statsConcurrency    = 8
)

// systemDatabases are the databases the server maintains itself. They are
// hidden while the cluster holds anything else; admin is kept because it holds
// users and roles and is the default authentication source.
var systemDatabases = map[string]bool{"config": true, "local": true}

// Conn is a live MongoDB client bound to one connection profile.
type Conn struct {
	client *mongo.Client
	cfg    models.ConnectionConfig

	mu     sync.Mutex
	closed bool
}

// compile time check: the driver implements the whole contract.
var _ drivers.Conn = (*Conn)(nil)

// Ping implements drivers.Conn.
func (c *Conn) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx, readpref.Primary()); err != nil {
		return apperr.Wrap(apperr.CodeConnectionFail, err, "ping MongoDB")
	}
	return nil
}

// Close implements drivers.Conn.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.Disconnect(ctx)
}

// Version implements drivers.Conn.
func (c *Conn) Version(ctx context.Context) (string, error) {
	var info struct {
		Version string `bson:"version"`
	}
	cmd := bson.D{{Key: "buildInfo", Value: 1}}
	if err := c.client.Database(defaultDatabase).RunCommand(ctx, cmd).Decode(&info); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read server version")
	}
	return info.Version, nil
}

// CurrentDatabase implements drivers.Conn.
func (c *Conn) CurrentDatabase(context.Context) (string, error) {
	return c.resolveDatabase(""), nil
}

// Databases implements drivers.Conn.
func (c *Conn) Databases(ctx context.Context) ([]string, error) {
	names, err := c.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	sort.Strings(names)

	// Keep the server's own databases only when there is nothing else: the
	// same rule the SQL drivers apply to their system schemas.
	visible := make([]string, 0, len(names))
	for _, name := range names {
		if !systemDatabases[name] {
			visible = append(visible, name)
		}
	}
	if len(visible) == 0 {
		return names, nil
	}
	return visible, nil
}

// Schemas implements drivers.Conn. MongoDB has no schemas: collections live
// directly in a database, so the explorer shows a single level.
func (c *Conn) Schemas(context.Context, string) ([]string, error) {
	return []string{}, nil
}

// Objects implements drivers.Conn.
func (c *Conn) Objects(ctx context.Context, database, schema string) ([]models.ObjectInfo, error) {
	db := c.database(database)
	specs, err := db.ListCollectionSpecifications(ctx, bson.D{})
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list collections")
	}

	out := make([]models.ObjectInfo, 0, len(specs))
	for _, spec := range specs {
		kind := models.KindCollection
		if spec.Type == "view" {
			kind = models.KindView
		}
		out = append(out, models.ObjectInfo{
			Name:     spec.Name,
			Database: db.Name(),
			Kind:     kind,
			Comment:  collectionComment(spec.Type, spec.Options),
			Engine:   spec.Type,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	if len(out) <= maxStatsCollections {
		c.fillStats(ctx, db, out)
	}
	return out, nil
}

// collectionSpecOptions is the subset of a collection's "options" document the
// UI shows as a comment.
type collectionSpecOptions struct {
	Capped     bool   `bson:"capped"`
	ViewOn     string `bson:"viewOn"`
	TimeSeries *struct {
		TimeField string `bson:"timeField"`
		MetaField string `bson:"metaField"`
	} `bson:"timeseries"`
	Validator bson.Raw `bson:"validator"`
}

// collectionComment explains what makes a collection special: a view, a time
// series collection, a capped collection or one with a schema validator.
//
// It takes the two fields it needs rather than the specification itself so the
// rules stay testable without constructing a driver type whose fields are
// unexported.
func collectionComment(specType string, options bson.Raw) string {
	var opts collectionSpecOptions
	// An options document is optional; a collection without one is ordinary.
	if err := bson.Unmarshal(options, &opts); err != nil {
		return ""
	}
	if specType == "view" {
		if opts.ViewOn == "" {
			return "view"
		}
		return "view on " + opts.ViewOn
	}
	var notes []string
	switch {
	case opts.TimeSeries != nil:
		note := "time series on " + opts.TimeSeries.TimeField
		if opts.TimeSeries.MetaField != "" {
			note += ", meta " + opts.TimeSeries.MetaField
		}
		notes = append(notes, note)
	case opts.Capped:
		notes = append(notes, "capped")
	}
	if len(opts.Validator) > 0 {
		notes = append(notes, "has a schema validator")
	}
	return strings.Join(notes, "; ")
}

// collectionStats is the trimmed $collStats result.
type collectionStats struct {
	Count        int64 `bson:"count"`
	StorageStats struct {
		Size        int64 `bson:"size"`
		StorageSize int64 `bson:"storageSize"`
		Count       int64 `bson:"count"`
	} `bson:"storageStats"`
}

// fillStats fills row counts and sizes for a whole namespace.
//
// It runs a bounded number of aggregations in parallel: a tree that shows "0
// rows" for every collection is useless, but a namespace with thousands of
// collections must not turn into thousands of round trips (Objects skips the
// work entirely past maxStatsCollections). Failures are ignored — a view does
// not answer $collStats, and a size nobody can read is not worth an error.
func (c *Conn) fillStats(ctx context.Context, db *mongo.Database, objects []models.ObjectInfo) {
	sem := make(chan struct{}, statsConcurrency)
	var wg sync.WaitGroup
	for i := range objects {
		if objects[i].Kind != models.KindCollection {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(target *models.ObjectInfo) {
			defer wg.Done()
			defer func() { <-sem }()
			stats, err := c.collectionStats(ctx, db.Name(), target.Name)
			if err != nil {
				return
			}
			target.RowEstimate = stats.Count
			target.SizeBytes = stats.StorageStats.Size
		}(&objects[i])
	}
	wg.Wait()
}

// collectionStats reads the document count and size of one collection.
func (c *Conn) collectionStats(ctx context.Context, database, collection string) (collectionStats, error) {
	var out collectionStats
	pipeline := bson.A{
		bson.D{{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{{Key: "scale", Value: 1}}}}}},
	}
	cursor, err := c.database(database).Collection(collection).Aggregate(ctx, pipeline)
	if err != nil {
		return out, err
	}
	defer cursor.Close(ctx)

	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return out, err
	}
	if len(docs) == 0 {
		return out, nil
	}
	if err := bson.Unmarshal(mustMarshal(docs[0]), &out); err != nil {
		return out, err
	}
	// Older servers (and views) report the count under storageStats only.
	if out.Count == 0 {
		out.Count = out.StorageStats.Count
	}
	return out, nil
}

// Structure implements drivers.Conn.
func (c *Conn) Structure(ctx context.Context, database, schema, object string) (*models.TableStructure, error) {
	db := c.database(database)
	coll := db.Collection(object)

	info, err := c.objectInfo(ctx, db, object)
	if err != nil {
		return nil, err
	}

	docs, err := c.sample(ctx, coll, fieldSampleSize)
	if err != nil {
		return nil, err
	}
	indexes, err := c.indexes(ctx, coll)
	if err != nil {
		return nil, err
	}

	if stats, err := c.collectionStats(ctx, db.Name(), object); err == nil && info.Kind == models.KindCollection {
		info.RowEstimate = stats.Count
		info.SizeBytes = stats.StorageStats.Size
	}

	return &models.TableStructure{
		Object:  info,
		Columns: inferColumns(docs),
		Indexes: indexes,
		// A document store has no foreign keys. The slice is empty rather than
		// nil so the UI can render it without a null check.
		ForeignKeys: []models.ForeignKeyInfo{},
		DDL:         definitionScript(db.Name(), object, indexes),
	}, nil
}

// objectInfo reads one collection's summary.
func (c *Conn) objectInfo(ctx context.Context, db *mongo.Database, object string) (models.ObjectInfo, error) {
	specs, err := db.ListCollectionSpecifications(ctx, bson.D{{Key: "name", Value: object}})
	if err != nil {
		return models.ObjectInfo{}, apperr.Wrap(apperr.CodeQueryFailed, err, "read collection %s", object)
	}
	if len(specs) == 0 {
		return models.ObjectInfo{}, apperr.New(apperr.CodeNotFound, "collection %s.%s does not exist", db.Name(), object)
	}
	spec := specs[0]
	kind := models.KindCollection
	if spec.Type == "view" {
		kind = models.KindView
	}
	return models.ObjectInfo{
		Name:     spec.Name,
		Database: db.Name(),
		Kind:     kind,
		Comment:  collectionComment(spec.Type, spec.Options),
		Engine:   spec.Type,
	}, nil
}

// sample reads the first documents of a collection, in a stable order so the
// inferred field list does not change between two reads.
func (c *Conn) sample(ctx context.Context, coll *mongo.Collection, limit int64) ([]bson.D, error) {
	opts := options.Find().SetLimit(limit).SetSort(bson.D{{Key: idField, Value: 1}})
	cursor, err := coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "sample documents from %s", coll.Name())
	}
	defer cursor.Close(ctx)

	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read documents from %s", coll.Name())
	}
	return docs, nil
}

// indexes reads the index catalog of one collection.
func (c *Conn) indexes(ctx context.Context, coll *mongo.Collection) ([]models.IndexInfo, error) {
	cursor, err := coll.Indexes().List(ctx)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes of %s", coll.Name())
	}
	defer cursor.Close(ctx)

	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read indexes of %s", coll.Name())
	}
	return indexInfos(docs), nil
}

// indexSpec is the subset of an index specification the UI shows.
type indexSpec struct {
	Name   string `bson:"name"`
	Key    bson.D `bson:"key"`
	Unique bool   `bson:"unique"`
	Hidden bool   `bson:"hidden"`
}

// indexInfos converts raw index specifications into the shared model.
func indexInfos(docs []bson.D) []models.IndexInfo {
	out := make([]models.IndexInfo, 0, len(docs))
	for _, doc := range docs {
		var spec indexSpec
		if err := bson.Unmarshal(mustMarshal(doc), &spec); err != nil {
			continue
		}
		out = append(out, indexInfo(spec))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Primary != out[j].Primary {
			return out[i].Primary
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func indexInfo(spec indexSpec) models.IndexInfo {
	columns := make([]string, 0, len(spec.Key))
	method := "btree"
	for _, elem := range spec.Key {
		columns = append(columns, elem.Key+" "+indexDirection(elem.Value))
		switch elem.Value {
		case "text":
			method = "text"
		case "2d", "2dsphere":
			method = "geo"
		case "hashed":
			method = "hashed"
		case "wildcard":
			method = "wildcard"
		}
	}
	name := spec.Name
	comment := ""
	if spec.Hidden {
		comment = "hidden"
	}
	return models.IndexInfo{
		Name:    name,
		Columns: columns,
		Unique:  spec.Unique,
		// "_id_" is the index MongoDB creates for the primary key field; it
		// cannot be dropped, which is exactly what the UI marks as primary.
		Primary: name == idField+"_",
		Method:  method,
		Comment: comment,
	}
}

// indexDirection renders the key direction of one index field.
func indexDirection(value any) string {
	switch v := value.(type) {
	case int32:
		if v < 0 {
			return "DESC"
		}
		return "ASC"
	case int64:
		if v < 0 {
			return "DESC"
		}
		return "ASC"
	case float64:
		if v < 0 {
			return "DESC"
		}
		return "ASC"
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// Indexes implements drivers.Conn: every index of every collection of a
// database, for the explorer's index folder.
//
// The folder carries a count like every other folder, so the explorer asks for
// this as soon as a database is expanded instead of waiting for a click. The
// per-collection reads therefore run with the same bounded concurrency as the
// collection statistics: a database with hundreds of collections must not turn
// one expansion into hundreds of sequential commands.
func (c *Conn) Indexes(ctx context.Context, database, schema string) ([]models.IndexEntry, error) {
	db := c.database(database)
	names, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list collections")
	}
	sort.Strings(names)

	// One slot per collection keeps the folder in name order whatever order the
	// reads come back in.
	groups := make([][]models.IndexEntry, len(names))
	sem := make(chan struct{}, statsConcurrency)
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func(slot int, collection string) {
			defer wg.Done()
			defer func() { <-sem }()
			indexes, err := c.indexes(ctx, db.Collection(collection))
			if err != nil {
				// A collection that vanished between the two calls, or one the
				// user cannot read: skip it, the rest of the folder is still
				// useful.
				return
			}
			entries := make([]models.IndexEntry, 0, len(indexes))
			for _, idx := range indexes {
				entries = append(entries, models.IndexEntry{
					Name:     idx.Name,
					Table:    collection,
					Database: db.Name(),
					Columns:  idx.Columns,
					Unique:   idx.Unique,
					Primary:  idx.Primary,
					Method:   idx.Method,
				})
			}
			groups[slot] = entries
		}(i, name)
	}
	wg.Wait()

	out := make([]models.IndexEntry, 0, len(names))
	for _, group := range groups {
		out = append(out, group...)
	}
	return out, nil
}

// Fetch implements drivers.Conn.
func (c *Conn) Fetch(ctx context.Context, req drivers.FetchRequest) (*models.FetchResult, error) {
	coll := c.database(req.Database).Collection(req.Object)

	filter, err := buildFilter(req.Filters)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "invalid filter")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := withTimeout(ctx, req.TimeoutMS)
	defer cancel()

	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(offset))
	// Stable paging: documents come back in natural order otherwise, which
	// repeats or skips rows across pages. _id is the only guaranteed index.
	sortSpec := buildSort(req.OrderBy)
	if len(sortSpec) == 0 {
		sortSpec = bson.D{{Key: idField, Value: 1}}
	}
	opts.SetSort(sortSpec)

	started := time.Now()
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "fetch documents")
	}
	defer cursor.Close(ctx)

	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read documents")
	}

	result := documentsToResult(docs)
	result.DurationMS = time.Since(started).Milliseconds()
	result.Truncated = int64(len(docs)) == int64(limit)
	result.SQL = describeFetch(req, filter, sortSpec, limit, offset)

	out := &models.FetchResult{QueryResult: *result}
	if req.CountTotal {
		if total, err := coll.CountDocuments(ctx, filter); err == nil {
			out.Total = total
			out.HasTotal = true
		}
		// A failed count is not fatal: the page is still useful.
	}
	return out, nil
}

// describeFetch renders the equivalent shell command, which is what the UI
// shows as "the query behind this page".
func describeFetch(req drivers.FetchRequest, filter bson.D, sort bson.D, limit, offset int) string {
	var sb strings.Builder
	sb.WriteString("db.getCollection(" + quoteText(req.Object) + ").find(")
	if len(filter) > 0 {
		sb.WriteString(describe(filter))
	} else {
		sb.WriteString("{}")
	}
	sb.WriteString(")")
	if len(sort) > 0 {
		sb.WriteString(".sort(" + describe(sort) + ")")
	}
	if offset > 0 {
		sb.WriteString(fmt.Sprintf(".skip(%d)", offset))
	}
	sb.WriteString(fmt.Sprintf(".limit(%d)", limit))
	return sb.String()
}

// UpdateCell implements drivers.Conn.
func (c *Conn) UpdateCell(ctx context.Context, req models.CellUpdate) (int64, error) {
	if strings.TrimSpace(req.Column) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "no field given")
	}
	return c.UpdateRow(ctx, models.RowUpdate{
		SessionID: req.SessionID,
		Database:  req.Database,
		Schema:    req.Schema,
		Object:    req.Object,
		Key:       req.Key,
		Values:    []models.KeyValue{{Column: req.Column, Value: req.Value}},
	})
}

// UpdateRow implements drivers.Conn: one $set covering every changed field.
//
// Only top level scalars can arrive here — the result grid offers nothing else
// as editable, and _id is immutable — so a change is one element per field.
func (c *Conn) UpdateRow(ctx context.Context, req models.RowUpdate) (int64, error) {
	set := bson.D{}
	for _, value := range req.Values {
		column := strings.TrimSpace(value.Column)
		if column == "" {
			return 0, apperr.New(apperr.CodeInvalidConfig, "no field given")
		}
		if column == idField {
			return 0, apperr.New(apperr.CodeInvalidConfig, "_id is immutable in MongoDB")
		}
		set = append(set, bson.E{Key: column, Value: writeValue(column, value.Value)})
	}
	if len(set) == 0 {
		return 0, apperr.New(apperr.CodeInvalidConfig, "no fields were changed")
	}
	filter, err := keyFilter(req.Key)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeInvalidConfig, err, "update row")
	}

	update := bson.D{{Key: "$set", Value: set}}
	res, err := c.database(req.Database).Collection(req.Object).UpdateOne(ctx, filter, update)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeQueryFailed, err, "update %s", req.Object)
	}
	return res.ModifiedCount, nil
}

// DeleteRow implements drivers.Conn.
func (c *Conn) DeleteRow(ctx context.Context, req models.RowDelete) (int64, error) {
	filter, err := keyFilter(req.Key)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeInvalidConfig, err, "delete row")
	}
	res, err := c.database(req.Database).Collection(req.Object).DeleteOne(ctx, filter)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeQueryFailed, err, "delete document")
	}
	return res.DeletedCount, nil
}

// PlanRowUpdate implements drivers.Conn: the shell command UpdateRow would run.
// The statement is rendered from the same filter and values, so the preview is
// what the change log keeps.
func (c *Conn) PlanRowUpdate(req models.RowUpdate) (string, error) {
	set := bson.D{}
	for _, value := range req.Values {
		column := strings.TrimSpace(value.Column)
		if column == "" {
			return "", apperr.New(apperr.CodeInvalidConfig, "no field given")
		}
		if column == idField {
			return "", apperr.New(apperr.CodeInvalidConfig, "_id is immutable in MongoDB")
		}
		set = append(set, bson.E{Key: column, Value: writeValue(column, value.Value)})
	}
	if len(set) == 0 {
		return "", apperr.New(apperr.CodeInvalidConfig, "no fields were changed")
	}
	filter, err := keyFilter(req.Key)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "update row")
	}
	return collectionRef(req.Object) + ".updateOne(" + describe(filter) + ", " +
		describe(bson.D{{Key: "$set", Value: set}}) + ")", nil
}

// PlanRowDelete implements drivers.Conn.
func (c *Conn) PlanRowDelete(req models.RowDelete) (string, error) {
	filter, err := keyFilter(req.Key)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInvalidConfig, err, "delete row")
	}
	return collectionRef(req.Object) + ".deleteOne(" + describe(filter) + ")", nil
}

// Execute implements drivers.Conn: it runs the shell language described in
// shell.go, statement by statement.
func (c *Conn) Execute(ctx context.Context, req drivers.ExecRequest) (*models.QueryResult, error) {
	statements := splitStatements(req.SQL)
	if len(statements) == 0 {
		return nil, apperr.New(apperr.CodeInvalidConfig, "no statement to execute")
	}

	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = defaultMaxRows
	}
	if maxRows > maxMaxRows {
		maxRows = maxMaxRows
	}

	ctx, cancel := withTimeout(ctx, req.TimeoutMS)
	defer cancel()

	// One database for the whole script, unless a `use` statement changes it:
	// the editor selects the first one, and `use` moves the statements that
	// follow it, exactly like the shell it is written for.
	database := c.resolveDatabase(req.Database)

	var (
		lastQuery *models.QueryResult
		lastExec  *models.QueryResult
		messages  []string
	)

	for i, raw := range statements {
		stmt, err := parseStatement(raw)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInvalidConfig, err, "statement %d", i+1)
		}
		if req.ReadOnly && stmt.isWrite() {
			return nil, apperr.New(apperr.CodeReadOnly, "connection is read-only, refusing to run statement %d (%s)", i+1, stmt.commandName())
		}

		// `use` never reaches the server: it moves the script to another
		// database. It is not a write either — nothing is created until the
		// first collection is — so a read-only session may still switch.
		if stmt.command == "use" {
			name, err := argString(argAt(stmt.args, 0))
			if err != nil || strings.TrimSpace(name) == "" {
				return nil, apperr.New(apperr.CodeInvalidConfig, "statement %d: use needs a database name", i+1)
			}
			database = strings.TrimSpace(name)
			messages = append(messages, fmt.Sprintf("#%d: switched to database %s", i+1, database))
			lastExec = affected(0, "switched to database "+database)
			lastExec.SQL = stmt.raw
			continue
		}

		started := time.Now()
		res, err := c.execStatement(ctx, database, stmt, maxRows)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "statement %d failed", i+1)
		}
		res.DurationMS = time.Since(started).Milliseconds()
		res.SQL = stmt.raw

		if res.HasResultSet {
			messages = append(messages, fmt.Sprintf("#%d: %d row(s) in %dms", i+1, res.RowCount, res.DurationMS))
			lastQuery = res
		} else {
			messages = append(messages, fmt.Sprintf("#%d: %d document(s) affected in %dms", i+1, res.AffectedRows, res.DurationMS))
			lastExec = res
		}
		// Messages a statement produced itself (an inserted id, an upserted
		// id) are part of the report; the final result overwrites Messages
		// with the whole list, so they have to be folded in here.
		messages = append(messages, res.Messages...)
	}

	// Prefer the last statement that produced a result set: that is what a
	// user running a script wants to look at.
	final := lastQuery
	if final == nil {
		final = lastExec
	}
	if final == nil {
		final = &models.QueryResult{Columns: []models.ColumnMeta{}, Rows: [][]any{}}
	}
	final.StatementCount = len(statements)
	final.StatementIndex = len(statements) - 1
	final.Messages = messages
	return final, nil
}

// Dialect implements drivers.Conn.
func (c *Conn) Dialect() drivers.Dialect { return Dialect{} }

// --- helpers ---------------------------------------------------------------

// resolveDatabase picks the database a bare statement or the session header
// refers to: the explicit argument first, then the profile, then admin.
func (c *Conn) resolveDatabase(database string) string {
	if strings.TrimSpace(database) != "" {
		return database
	}
	if strings.TrimSpace(c.cfg.Database) != "" {
		return c.cfg.Database
	}
	return defaultDatabase
}

func (c *Conn) database(name string) *mongo.Database {
	return c.client.Database(c.resolveDatabase(name))
}

// mustMarshal panics on a value that cannot be marshalled, which can only
// happen for a document the server just sent us.
func mustMarshal(value any) []byte {
	raw, err := bson.Marshal(value)
	if err != nil {
		panic("mongodb: marshal " + err.Error())
	}
	return raw
}

// withTimeout applies the per-request timeout, defaulting to a minute so a
// forgotten query cannot wedge the UI forever.
func withTimeout(ctx context.Context, ms int) (context.Context, context.CancelFunc) {
	if ms <= 0 {
		ms = defaultTimeoutMS
	}
	return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
}

// quoteText renders a JSON string literal, for generated shell commands.
func quoteText(value string) string {
	raw, err := bson.MarshalExtJSON(value, false, false)
	if err != nil {
		return fmt.Sprintf("%q", value)
	}
	return string(raw)
}

// definitionScript renders the shell equivalent of a CREATE statement.
//
// MongoDB has no DDL, so the closest thing is the script that would recreate
// the collection: createCollection (with nothing to say for an ordinary
// collection) plus its indexes. It is valid input for the shell language, so
// the DDL editor can run it back.
func definitionScript(database, object string, indexes []models.IndexInfo) string {
	lines := []string{
		"// Definition of " + database + "." + object,
	}
	if len(indexes) == 0 {
		// A view, or a collection whose index list could not be read. The
		// shell has no "CREATE", only createCollection, so that is what a
		// replay starts from.
		lines = append(lines, fmt.Sprintf("db.createCollection(%s);", quoteText(object)))
		return strings.Join(lines, "\n") + "\n"
	}
	for _, idx := range indexes {
		if idx.Primary {
			// _id_ is created with the collection, it cannot be recreated.
			continue
		}
		lines = append(lines, fmt.Sprintf("// index %s (%s)", idx.Name, strings.Join(idx.Columns, ", ")))
		lines = append(lines, fmt.Sprintf("%s.createIndex(%s, %s);",
			collectionRef(object), describe(indexKey(idx)), describe(indexOptions(idx))))
	}
	return strings.Join(lines, "\n") + "\n"
}

// collectionRef renders a collection reference that the shell language accepts
// whatever characters the name contains.
func collectionRef(object string) string {
	if isPlainName(object) {
		return "db." + object
	}
	return "db.getCollection(" + quoteText(object) + ")"
}

func isPlainName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// indexKey rebuilds the key document of an index from its rendered columns.
func indexKey(idx models.IndexInfo) bson.D {
	key := make(bson.D, 0, len(idx.Columns))
	for _, column := range idx.Columns {
		name, direction := column, "ASC"
		if at := strings.LastIndex(column, " "); at > 0 {
			name, direction = column[:at], column[at+1:]
		}
		switch direction {
		case "DESC":
			key = append(key, bson.E{Key: name, Value: -1})
		case "ASC":
			key = append(key, bson.E{Key: name, Value: 1})
		default:
			key = append(key, bson.E{Key: name, Value: direction})
		}
	}
	return key
}

// indexOptions rebuilds the option document of an index.
func indexOptions(idx models.IndexInfo) bson.D {
	out := bson.D{{Key: "name", Value: idx.Name}}
	if idx.Unique {
		out = append(out, bson.E{Key: "unique", Value: true})
	}
	if idx.Comment == "hidden" {
		out = append(out, bson.E{Key: "hidden", Value: true})
	}
	return out
}
