package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
)

// Handle management
var (
	handleMu   sync.RWMutex
	handles    = make(map[uint64]*handleEntry)
	nextHandle uint64 = 1
)

type handleEntry struct {
	db          *sql.DB
	project     core.Project
	projectPath string
}

func registerHandle(database *sql.DB, project core.Project, projectPath string) uint64 {
	handleMu.Lock()
	defer handleMu.Unlock()
	id := nextHandle
	nextHandle++
	handles[id] = &handleEntry{
		db:          database,
		project:     project,
		projectPath: projectPath,
	}
	return id
}

func getHandle(id uint64) (*handleEntry, bool) {
	handleMu.RLock()
	defer handleMu.RUnlock()
	entry, ok := handles[id]
	return entry, ok
}

func closeHandle(id uint64) {
	handleMu.Lock()
	defer handleMu.Unlock()
	if entry, ok := handles[id]; ok {
		if entry.db != nil {
			_ = entry.db.Close()
		}
		delete(handles, id)
	}
}

// JSON response types
type Response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error *string     `json:"error,omitempty"`
}

type MessagePageResponse struct {
	Messages []interface{}   `json:"messages"`
	Cursor   *CursorResponse `json:"cursor,omitempty"`
}

type CursorResponse struct {
	GUID string `json:"guid"`
	TS   int64  `json:"ts"`
}

type ProjectResponse struct {
	Root   string `json:"root"`
	DBPath string `json:"db_path"`
}

func successResponse(data interface{}) []byte {
	resp := Response{OK: true, Data: data}
	bytes, err := json.Marshal(resp)
	if err != nil {
		return errorResponse("failed to marshal response: " + err.Error())
	}
	return bytes
}

func errorResponse(errMsg string) []byte {
	resp := Response{OK: false, Error: &errMsg}
	bytes, _ := json.Marshal(resp)
	return bytes
}

// Helper functions
func goStringToC(s string) *C.char {
	return C.CString(s)
}

func cStringToGo(cs *C.char) string {
	if cs == nil {
		return ""
	}
	return C.GoString(cs)
}

func returnJSON(data []byte) *C.char {
	return goStringToC(string(data))
}

func parseCursor(cursorStr string) (*types.MessageCursor, error) {
	if cursorStr == "" {
		return nil, nil
	}
	parts := strings.SplitN(cursorStr, ":", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, nil
	}
	return &types.MessageCursor{GUID: parts[0], TS: ts}, nil
}

func messagesToInterface(messages []types.Message) []interface{} {
	result := make([]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = msg
	}
	return result
}

// FFI Exports

//export FrayDiscoverProject
func FrayDiscoverProject(startDir *C.char) *C.char {
	dir := cStringToGo(startDir)
	project, err := core.DiscoverProject(dir)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	return returnJSON(successResponse(ProjectResponse{
		Root:   project.Root,
		DBPath: project.DBPath,
	}))
}

//export FrayOpenDatabase
func FrayOpenDatabase(projectPath *C.char) C.ulonglong {
	path := cStringToGo(projectPath)
	project, err := core.DiscoverProject(path)
	if err != nil {
		return 0
	}
	database, err := db.OpenDatabase(project)
	if err != nil {
		return 0
	}
	return C.ulonglong(registerHandle(database, project, path))
}

//export FrayCloseDatabase
func FrayCloseDatabase(handle C.ulonglong) {
	closeHandle(uint64(handle))
}

//export FrayGetMessages
func FrayGetMessages(handle C.ulonglong, home *C.char, limit C.int, sinceCursor *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	homeStr := cStringToGo(home)
	cursorStr := cStringToGo(sinceCursor)

	cursor, _ := parseCursor(cursorStr)

	var homePtr *string
	if home != nil {
		homePtr = &homeStr
	}

	opts := &types.MessageQueryOptions{
		Limit: int(limit),
		Since: cursor,
		Home:  homePtr,
	}

	messages, err := db.GetMessages(entry.db, opts)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	var nextCursor *CursorResponse
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		nextCursor = &CursorResponse{GUID: last.ID, TS: last.TS}
	}

	return returnJSON(successResponse(MessagePageResponse{
		Messages: messagesToInterface(messages),
		Cursor:   nextCursor,
	}))
}

//export FrayPostMessage
func FrayPostMessage(handle C.ulonglong, body, fromAgent, home, replyTo *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	bodyStr := cStringToGo(body)
	agentStr := cStringToGo(fromAgent)
	homeStr := cStringToGo(home)
	replyToStr := cStringToGo(replyTo)

	// Resolve thread name to GUID if needed
	if homeStr != "" && homeStr != "room" {
		thread, err := db.GetThreadByNameAny(entry.db, homeStr)
		if err == nil && thread != nil {
			homeStr = thread.GUID
		}
	}

	// Determine message type: user for humans, agent for managed agents
	msgType := types.MessageTypeUser
	agent, _ := db.GetAgent(entry.db, agentStr)
	if agent != nil && agent.Managed {
		msgType = types.MessageTypeAgent
	}

	mentions := core.ExtractMentions(bodyStr, nil)

	msg := types.Message{
		Body:      bodyStr,
		FromAgent: agentStr,
		Home:      homeStr,
		Mentions:  mentions,
		Type:      msgType,
	}

	if replyToStr != "" {
		msg.ReplyTo = &replyToStr
	}

	created, err := db.CreateMessage(entry.db, msg)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	frayDir := filepath.Dir(entry.project.DBPath)
	if err := db.AppendMessage(frayDir, created); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	now := time.Now().Unix()
	_ = db.UpdateAgent(entry.db, agentStr, db.AgentUpdates{
		LastSeen: types.OptionalInt64{Set: true, Value: &now},
	})

	return returnJSON(successResponse(created))
}

//export FrayEditMessage
func FrayEditMessage(handle C.ulonglong, msgID, newBody, reason *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	msgIDStr := cStringToGo(msgID)
	newBodyStr := cStringToGo(newBody)
	reasonStr := cStringToGo(reason)

	msg, err := db.GetMessage(entry.db, msgIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if msg == nil {
		return returnJSON(errorResponse("message not found"))
	}

	if err := db.EditMessage(entry.db, msgIDStr, newBodyStr, msg.FromAgent); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	frayDir := filepath.Dir(entry.project.DBPath)
	var reasonPtr *string
	if reasonStr != "" {
		reasonPtr = &reasonStr
	}
	_ = db.AppendMessageUpdate(frayDir, db.MessageUpdateJSONLRecord{
		ID:     msgIDStr,
		Body:   &newBodyStr,
		Reason: reasonPtr,
	})

	updatedMsg, err := db.GetMessage(entry.db, msgIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(updatedMsg))
}

//export FrayAddReaction
func FrayAddReaction(handle C.ulonglong, msgID, emoji, agent *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	msgIDStr := cStringToGo(msgID)
	emojiStr := cStringToGo(emoji)
	agentStr := cStringToGo(agent)

	msg, reactedAt, err := db.AddReaction(entry.db, msgIDStr, agentStr, emojiStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	frayDir := filepath.Dir(entry.project.DBPath)
	_ = db.AppendReaction(frayDir, msgIDStr, agentStr, emojiStr, reactedAt)

	return returnJSON(successResponse(msg))
}

//export FrayGetAgents
func FrayGetAgents(handle C.ulonglong, managedOnly C.int) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	var agents []types.Agent
	var err error

	if managedOnly != 0 {
		agents, err = db.GetManagedAgents(entry.db)
	} else {
		agents, err = db.GetAgents(entry.db)
	}
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(agents))
}

//export FrayGetAgent
func FrayGetAgent(handle C.ulonglong, agentID *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	agentIDStr := cStringToGo(agentID)
	agent, err := db.GetAgent(entry.db, agentIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if agent == nil {
		return returnJSON(errorResponse("agent not found"))
	}

	return returnJSON(successResponse(agent))
}

//export FrayGetThreads
func FrayGetThreads(handle C.ulonglong, parentThread *C.char, includeArchived C.int) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	var parentPtr *string
	if parentThread != nil {
		parentStr := cStringToGo(parentThread)
		if parentStr != "" {
			parentPtr = &parentStr
		}
	}

	opts := &types.ThreadQueryOptions{
		ParentThread:    parentPtr,
		IncludeArchived: includeArchived != 0,
	}

	threads, err := db.GetThreads(entry.db, opts)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(threads))
}

//export FrayGetThread
func FrayGetThread(handle C.ulonglong, threadRef *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	threadRefStr := cStringToGo(threadRef)
	if threadRefStr == "" {
		return returnJSON(errorResponse("thread reference required"))
	}

	// Try by GUID first
	if strings.HasPrefix(threadRefStr, "thrd-") {
		thread, err := db.GetThread(entry.db, threadRefStr)
		if err != nil {
			return returnJSON(errorResponse(err.Error()))
		}
		if thread != nil {
			return returnJSON(successResponse(thread))
		}
	}

	// Try by prefix
	thread, err := db.GetThreadByPrefix(entry.db, threadRefStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if thread != nil {
		return returnJSON(successResponse(thread))
	}

	// Try by name
	thread, err = db.GetThreadByNameAny(entry.db, threadRefStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if thread == nil {
		return returnJSON(errorResponse("thread not found"))
	}

	return returnJSON(successResponse(thread))
}

//export FrayGetThreadMessages
func FrayGetThreadMessages(handle C.ulonglong, threadGUID *C.char, limit C.int, sinceCursor *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	threadGUIDStr := cStringToGo(threadGUID)
	cursorStr := cStringToGo(sinceCursor)

	cursor, _ := parseCursor(cursorStr)

	messages, err := db.GetThreadMessages(entry.db, threadGUIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	// Apply cursor filtering in memory
	if cursor != nil {
		filtered := make([]types.Message, 0, len(messages))
		for _, msg := range messages {
			if msg.TS > cursor.TS || (msg.TS == cursor.TS && msg.ID > cursor.GUID) {
				filtered = append(filtered, msg)
			}
		}
		messages = filtered
	}

	// Apply limit
	if limit > 0 && int(limit) < len(messages) {
		messages = messages[:int(limit)]
	}

	var nextCursor *CursorResponse
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		nextCursor = &CursorResponse{GUID: last.ID, TS: last.TS}
	}

	return returnJSON(successResponse(MessagePageResponse{
		Messages: messagesToInterface(messages),
		Cursor:   nextCursor,
	}))
}

//export FraySubscribeToThread
func FraySubscribeToThread(handle C.ulonglong, threadGUID, agentID *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	threadGUIDStr := cStringToGo(threadGUID)
	agentIDStr := cStringToGo(agentID)

	if err := db.SubscribeThread(entry.db, threadGUIDStr, agentIDStr, 0); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	frayDir := filepath.Dir(entry.project.DBPath)
	_ = db.AppendThreadSubscribe(frayDir, db.ThreadSubscribeJSONLRecord{
		ThreadGUID:   threadGUIDStr,
		AgentID:      agentIDStr,
		SubscribedAt: time.Now().Unix(),
	})

	return returnJSON(successResponse(map[string]bool{"subscribed": true}))
}

//export FrayFaveItem
func FrayFaveItem(handle C.ulonglong, itemGUID, agentID *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	itemGUIDStr := cStringToGo(itemGUID)
	agentIDStr := cStringToGo(agentID)

	itemType := "message"
	if strings.HasPrefix(itemGUIDStr, "thrd-") {
		itemType = "thread"
	}

	favedAt, err := db.AddFave(entry.db, agentIDStr, itemType, itemGUIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(map[string]interface{}{
		"faved":    true,
		"faved_at": favedAt,
	}))
}

//export FrayGetReadTo
func FrayGetReadTo(handle C.ulonglong, agentID, home *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	agentIDStr := cStringToGo(agentID)
	homeStr := cStringToGo(home)

	readTo, err := db.GetReadTo(entry.db, agentIDStr, homeStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(readTo))
}

//export FraySetReadTo
func FraySetReadTo(handle C.ulonglong, agentID, home, msgID *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	agentIDStr := cStringToGo(agentID)
	homeStr := cStringToGo(home)
	msgIDStr := cStringToGo(msgID)

	msg, err := db.GetMessage(entry.db, msgIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if msg == nil {
		return returnJSON(errorResponse("message not found"))
	}

	if err := db.SetReadTo(entry.db, agentIDStr, homeStr, msgIDStr, msg.TS); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(map[string]bool{"set": true}))
}

//export FrayRegisterAgent
func FrayRegisterAgent(handle C.ulonglong, agentID *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	agentIDStr := cStringToGo(agentID)
	if agentIDStr == "" {
		return returnJSON(errorResponse("agent ID required"))
	}

	if !core.IsValidAgentID(agentIDStr) {
		return returnJSON(errorResponse("invalid agent ID: must start with lowercase letter and contain only lowercase letters, numbers, hyphens, and dots"))
	}

	existing, err := db.GetAgent(entry.db, agentIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}
	if existing != nil {
		return returnJSON(successResponse(existing))
	}

	agentGUID, err := core.GenerateGUID("usr")
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	usedAvatars := make(map[string]struct{})
	existingAgents, _ := db.GetAgents(entry.db)
	for _, a := range existingAgents {
		if a.Avatar != nil && *a.Avatar != "" {
			usedAvatars[*a.Avatar] = struct{}{}
		}
	}
	avatar := core.AssignAvatar(agentIDStr, usedAvatars)

	now := time.Now().Unix()
	agent := types.Agent{
		GUID:         agentGUID,
		AgentID:      agentIDStr,
		Avatar:       &avatar,
		RegisteredAt: now,
		LastSeen:     now,
		Presence:     types.PresenceOffline,
	}

	if err := db.CreateAgent(entry.db, agent); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	frayDir := filepath.Dir(entry.project.DBPath)
	if err := db.AppendAgent(frayDir, agent); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	created, err := db.GetAgent(entry.db, agentIDStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(created))
}

//export FrayGetConfig
func FrayGetConfig(handle C.ulonglong, key *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	keyStr := cStringToGo(key)
	if keyStr == "" {
		return returnJSON(errorResponse("key required"))
	}

	value, err := db.GetConfig(entry.db, keyStr)
	if err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(value))
}

//export FraySetConfig
func FraySetConfig(handle C.ulonglong, key, value *C.char) *C.char {
	entry, ok := getHandle(uint64(handle))
	if !ok {
		return returnJSON(errorResponse("invalid database handle"))
	}

	keyStr := cStringToGo(key)
	valueStr := cStringToGo(value)
	if keyStr == "" {
		return returnJSON(errorResponse("key required"))
	}

	if err := db.SetConfig(entry.db, keyStr, valueStr); err != nil {
		return returnJSON(errorResponse(err.Error()))
	}

	return returnJSON(successResponse(map[string]bool{"set": true}))
}

//export FrayFreeString
func FrayFreeString(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

func main() {}
