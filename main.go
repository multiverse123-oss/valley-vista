package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

func main() {
	app := pocketbase.New()

	// CORS
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
		}))
		return nil
	})

	// Ensure collections synchronously before serving
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		if err := ensureCollections(app); err != nil {
			log.Printf("Error ensuring collections: %v", err)
			return err
		}
		if err := applyS3FromEnv(app); err != nil {
			log.Printf("Error applying S3 settings from env: %v", err)
			// Do not fail startup — we still want PocketBase to run
		}
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// applyS3FromEnv re-applies S3 file storage settings on every boot.
// This survives Litestream rollbacks because the config lives in Render env vars,
// not in the database.
func applyS3FromEnv(app *pocketbase.PocketBase) error {
	if os.Getenv("VV_S3_ENABLED") != "true" {
		log.Println("[startup] VV_S3_ENABLED not set to true — skipping S3 config")
		return nil
	}

	settings, err := app.Dao().FindSettings()
	if err != nil {
		return err
	}

	settings.S3.Enabled = true
	settings.S3.Bucket = os.Getenv("VV_S3_BUCKET")
	settings.S3.Region = os.Getenv("VV_S3_REGION")
	settings.S3.Endpoint = os.Getenv("VV_S3_ENDPOINT")
	settings.S3.AccessKey = os.Getenv("VV_S3_ACCESS_KEY")
	settings.S3.Secret = os.Getenv("VV_S3_SECRET")
	settings.S3.ForcePathStyle = true

	if err := app.Dao().SaveSettings(settings); err != nil {
		return err
	}
	log.Printf("[startup] ✅ S3 settings applied from env — bucket: %s", settings.S3.Bucket)
	return nil
}

func ensureCollections(app *pocketbase.PocketBase) error {
	dao := app.Dao()
	if dao == nil {
		return nil
	}

	// 1. Add isAdmin to users
	usersCollection, err := dao.FindCollectionByNameOrId("users")
	if err != nil || usersCollection == nil {
		log.Printf("Warning: users collection not found: %v", err)
		return nil
	}
	if !hasField(usersCollection, "isAdmin") {
		usersCollection.Schema.AddField(&schema.SchemaField{
			Name:     "isAdmin",
			Type:     schema.FieldTypeBool,
			Required: false,
			Options:  &schema.BoolOptions{},
		})
		if err := dao.SaveCollection(usersCollection); err != nil {
			return err
		}
	}
	if !hasField(usersCollection, "role") {
		usersCollection.Schema.AddField(&schema.SchemaField{
			Name:     "role",
			Type:     schema.FieldTypeSelect,
			Required: false,
			Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values:    []string{"student", "agent", "admin", "user"},
			},
		})
		if err := dao.SaveCollection(usersCollection); err != nil {
			return err
		}
	}

	// 2. properties
	if _, err := dao.FindCollectionByNameOrId("properties"); err != nil {
		collection := &models.Collection{
			Name:       "properties",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "title", Type: schema.FieldTypeText, Required: true},
			{Name: "description", Type: schema.FieldTypeText, Required: false},
			{Name: "address", Type: schema.FieldTypeText, Required: true},
			{Name: "price", Type: schema.FieldTypeNumber, Required: true},
			{Name: "type", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"house", "warehouse", "apartment", "villa"}}},
			{Name: "listing_type", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"sale", "rent"}}},
			{Name: "status", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"available", "sold", "rented"}}},
			{Name: "bedrooms", Type: schema.FieldTypeNumber, Required: false},
			{Name: "bathrooms", Type: schema.FieldTypeNumber, Required: false},
			{Name: "size", Type: schema.FieldTypeNumber, Required: false},
			{Name: "latitude", Type: schema.FieldTypeNumber, Required: false},
			{Name: "longitude", Type: schema.FieldTypeNumber, Required: false},
			{Name: "formatted_address", Type: schema.FieldTypeText, Required: false},
			{Name: "place_id", Type: schema.FieldTypeText, Required: false},
			{Name: "geocoded_at", Type: schema.FieldTypeDate, Required: false},
			{Name: "geocoding_status", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"pending", "success", "failed", "manual"}}},
			{Name: "location_accuracy", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"exact", "street", "area", "city", "approximate"}}},
			{Name: "map_zoom", Type: schema.FieldTypeNumber, Required: false},
			{Name: "location_notes", Type: schema.FieldTypeText, Required: false},
			{Name: "nearest_landmark", Type: schema.FieldTypeText, Required: false},
			{Name: "satellite_override", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 10485760, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}}},
			{Name: "images", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 10, MaxSize: 5242880, MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"}}},
			{Name: "video", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 52428800, MimeTypes: []string{"video/mp4", "video/webm", "video/quicktime"}}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	} else {
		// Property collection exists — ensure location fields are present
		propCol, _ := dao.FindCollectionByNameOrId("properties")
		if propCol != nil {
			added := false
			for _, name := range []string{"formatted_address", "place_id", "geocoded_at", "geocoding_status", "location_accuracy", "map_zoom", "location_notes", "nearest_landmark"} {
				if !hasField(propCol, name) {
					propCol.Schema.AddField(&schema.SchemaField{Name: name, Type: schema.FieldTypeText, Required: false})
					added = true
				}
			}
			if !hasField(propCol, "satellite_override") {
				propCol.Schema.AddField(&schema.SchemaField{Name: "satellite_override", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 10485760, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}}})
				added = true
			}
			if added {
				if err := dao.SaveCollection(propCol); err != nil {
					log.Printf("Warning: could not add location fields to properties: %v", err)
				} else {
					log.Println("[startup] Added missing location fields to properties")
				}
			}
		}
	}

	// 3. contact_requests
	if _, err := dao.FindCollectionByNameOrId("contact_requests"); err != nil {
		collection := &models.Collection{
			Name:       "contact_requests",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "property", Type: schema.FieldTypeRelation, Required: false, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "name", Type: schema.FieldTypeText, Required: true},
			{Name: "email", Type: schema.FieldTypeEmail, Required: false},
			{Name: "phone", Type: schema.FieldTypeText, Required: false},
			{Name: "subject", Type: schema.FieldTypeText, Required: false},
			{Name: "message", Type: schema.FieldTypeText, Required: true},
			{Name: "status", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"open", "replied", "closed"}}},
			{Name: "replied_at", Type: schema.FieldTypeDate, Required: false},
			{Name: "reply_count", Type: schema.FieldTypeNumber, Required: false},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		propCollection, _ := dao.FindCollectionByNameOrId("properties")
		if propCollection != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "property" {
					f.Options.(*schema.RelationOptions).CollectionId = propCollection.Id
				}
			}
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 4. property_requests
	if _, err := dao.FindCollectionByNameOrId("property_requests"); err != nil {
		collection := &models.Collection{
			Name:       "property_requests",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			ViewRule:   types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "user", Type: schema.FieldTypeRelation, Required: false, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "description", Type: schema.FieldTypeText, Required: true},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		usersColl, _ := dao.FindCollectionByNameOrId("users")
		if usersColl != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "user" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl.Id
				}
			}
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 5. owner_enquiries
	if _, err := dao.FindCollectionByNameOrId("owner_enquiries"); err != nil {
		collection := &models.Collection{
			Name:       "owner_enquiries",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			ViewRule:   types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "full_name", Type: schema.FieldTypeText, Required: true},
			{Name: "company_name", Type: schema.FieldTypeText, Required: false},
			{Name: "phone", Type: schema.FieldTypeText, Required: true},
			{Name: "property_address", Type: schema.FieldTypeText, Required: true},
			{Name: "category", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"general_enquiry", "inspection", "filming"}}},
			{Name: "message", Type: schema.FieldTypeText, Required: true},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 6. professionals
	if _, err := dao.FindCollectionByNameOrId("professionals"); err != nil {
		collection := &models.Collection{
			Name:       "professionals",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "name", Type: schema.FieldTypeText, Required: true},
			{Name: "role", Type: schema.FieldTypeText, Required: true},
			{Name: "company", Type: schema.FieldTypeText, Required: false},
			{Name: "email", Type: schema.FieldTypeEmail, Required: false},
			{Name: "bio", Type: schema.FieldTypeText, Required: false},
			{Name: "category", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"architect", "builder", "contractor", "roofer", "plasterer", "painter", "engineer", "surveyor", "interior_designer", "landscaper", "electrician", "plumber"}}},
			{Name: "avatar", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 2097152, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}}},
			{Name: "logo", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 2097152, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	} else {
		// Ensure category field exists
		profCol, _ := dao.FindCollectionByNameOrId("professionals")
		if profCol != nil && !hasField(profCol, "category") {
			profCol.Schema.AddField(&schema.SchemaField{
				Name: "category", Type: schema.FieldTypeSelect, Required: false,
				Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"architect", "builder", "contractor", "roofer", "plasterer", "painter", "engineer", "surveyor", "interior_designer", "landscaper", "electrician", "plumber"}},
			})
			if err := dao.SaveCollection(profCol); err != nil {
				log.Printf("Warning: could not add category to professionals: %v", err)
			} else {
				log.Println("[startup] Added category field to professionals")
			}
		}
	}

	// 7. favorites
	if _, err := dao.FindCollectionByNameOrId("favorites"); err != nil {
		collection := &models.Collection{
			Name:       "favorites",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.id != '' && user = @request.auth.id"),
			ViewRule:   types.Pointer("@request.auth.id != '' && user = @request.auth.id"),
			CreateRule: types.Pointer("@request.auth.id != '' && user = @request.auth.id"),
			UpdateRule: types.Pointer("@request.auth.id != '' && user = @request.auth.id"),
			DeleteRule: types.Pointer("@request.auth.id != '' && user = @request.auth.id"),
		}
		fields := []*schema.SchemaField{
			{Name: "user", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "property", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		usersColl2, _ := dao.FindCollectionByNameOrId("users")
		propColl2, _ := dao.FindCollectionByNameOrId("properties")
		if usersColl2 != nil && propColl2 != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "user" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl2.Id
				}
				if f.Name == "property" {
					f.Options.(*schema.RelationOptions).CollectionId = propColl2.Id
				}
			}
		}
		collection.Indexes = types.JsonArray[string]{"CREATE UNIQUE INDEX idx_favorites_user_property ON favorites (user, property)"}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 8. gallery_images
	if _, err := dao.FindCollectionByNameOrId("gallery_images"); err != nil {
		collection := &models.Collection{
			Name:       "gallery_images",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "title", Type: schema.FieldTypeText, Required: false},
			{Name: "caption", Type: schema.FieldTypeText, Required: false},
			{Name: "description", Type: schema.FieldTypeText, Required: false},
			{Name: "album", Type: schema.FieldTypeText, Required: false},
			{Name: "category", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"campus", "events", "graduation", "sports", "academics", "facilities", "other"}}},
			{Name: "display_order", Type: schema.FieldTypeNumber, Required: false},
			{Name: "is_published", Type: schema.FieldTypeBool, Required: false},
			{Name: "image", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 1, MaxSize: 5242880, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 9. contact_replies
	if _, err := dao.FindCollectionByNameOrId("contact_replies"); err != nil {
		crCol, _ := dao.FindCollectionByNameOrId("contact_requests")
		collection := &models.Collection{
			Name:       "contact_replies",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "contact_request", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "admin_sender", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "subject", Type: schema.FieldTypeText, Required: false},
			{Name: "message", Type: schema.FieldTypeText, Required: true},
			{Name: "is_read", Type: schema.FieldTypeBool, Required: false},
			{Name: "read_at", Type: schema.FieldTypeDate, Required: false},
			{Name: "attachments", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 5, MaxSize: 52428800, MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/webm", "video/quicktime"}}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if crCol != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "contact_request" {
					f.Options.(*schema.RelationOptions).CollectionId = crCol.Id
				}
			}
		}
		if usersColl != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "admin_sender" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl.Id
				}
			}
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 10. support_chats
	if _, err := dao.FindCollectionByNameOrId("support_chats"); err != nil {
		collection := &models.Collection{
			Name:       "support_chats",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer(""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "visitor_id", Type: schema.FieldTypeText, Required: false},
			{Name: "user", Type: schema.FieldTypeRelation, Required: false, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "visitor_name", Type: schema.FieldTypeText, Required: false},
			{Name: "visitor_email", Type: schema.FieldTypeEmail, Required: false},
			{Name: "subject", Type: schema.FieldTypeText, Required: false},
			{Name: "status", Type: schema.FieldTypeSelect, Required: false, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"active", "waiting_for_admin", "resolved", "closed"}}},
			{Name: "last_message_at", Type: schema.FieldTypeDate, Required: false},
			{Name: "unread_for_admin", Type: schema.FieldTypeNumber, Required: false},
			{Name: "unread_for_visitor", Type: schema.FieldTypeNumber, Required: false},
			{Name: "ai_enabled", Type: schema.FieldTypeBool, Required: false},
			{Name: "page_url", Type: schema.FieldTypeText, Required: false},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if usersColl != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "user" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl.Id
				}
			}
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 11. support_chat_messages
	if _, err := dao.FindCollectionByNameOrId("support_chat_messages"); err != nil {
		scCol, _ := dao.FindCollectionByNameOrId("support_chats")
		collection := &models.Collection{
			Name:       "support_chat_messages",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer(""),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true || @request.auth.role = \"admin\""),
		}
		fields := []*schema.SchemaField{
			{Name: "chat", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "sender_type", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{MaxSelect: 1, Values: []string{"visitor", "ai", "admin"}}},
			{Name: "sender", Type: schema.FieldTypeRelation, Required: false, Options: &schema.RelationOptions{CollectionId: "", MaxSelect: types.Pointer(1)}},
			{Name: "sender_name", Type: schema.FieldTypeText, Required: false},
			{Name: "message", Type: schema.FieldTypeText, Required: false},
			{Name: "is_read", Type: schema.FieldTypeBool, Required: false},
			{Name: "read_at", Type: schema.FieldTypeDate, Required: false},
			{Name: "attachments", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{MaxSelect: 5, MaxSize: 52428800, MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/webm", "video/quicktime"}}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if scCol != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "chat" {
					f.Options.(*schema.RelationOptions).CollectionId = scCol.Id
				}
			}
		}
		if usersColl != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "sender" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl.Id
				}
			}
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// Create PocketBase superuser using DAO
	pbAdminEmail := os.Getenv("PB_ADMIN_EMAIL")
	pbAdminPassword := os.Getenv("PB_ADMIN_PASSWORD")
	if pbAdminEmail == "" || pbAdminPassword == "" {
		log.Println("Warning: PB_ADMIN_EMAIL or PB_ADMIN_PASSWORD not set. Superuser will not be created automatically.")
	} else {
		_, err := dao.FindAdminByEmail(pbAdminEmail)
		if err != nil {
			newAdmin := &models.Admin{}
			newAdmin.Email = pbAdminEmail
			newAdmin.SetPassword(pbAdminPassword)
			if saveErr := dao.SaveAdmin(newAdmin); saveErr != nil {
				log.Printf("Warning: could not create PocketBase superuser: %v", saveErr)
			} else {
				log.Printf("PocketBase superuser created: %s", pbAdminEmail)
			}
		} else {
			log.Printf("PocketBase superuser already exists: %s", pbAdminEmail)
		}
	}

	// Create regular admin user if env vars set
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminEmail == "" || adminPassword == "" {
		log.Println("Warning: ADMIN_EMAIL or ADMIN_PASSWORD not set. Regular admin user will not be created automatically.")
	} else {
		log.Printf("Checking for existing admin user: %s", adminEmail)
		user, err := dao.FindAuthRecordByEmail("users", adminEmail)
		if err != nil {
			collection, colErr := dao.FindCollectionByNameOrId("users")
			if colErr != nil || collection == nil {
				log.Printf("Error: users collection not found: %v", colErr)
			} else {
				newUser := models.NewRecord(collection)
				newUser.Set("email", adminEmail)
				newUser.Set("username", usernameFromEmail(adminEmail))
				newUser.Set("password", adminPassword)
				newUser.Set("passwordConfirm", adminPassword)
				newUser.Set("isAdmin", true)
				newUser.Set("role", "admin")
				newUser.Set("verified", true)
				if saveErr := dao.SaveRecord(newUser); saveErr != nil {
					if strings.Contains(saveErr.Error(), "duplicate") || strings.Contains(saveErr.Error(), "UNIQUE") {
						log.Printf("Admin user already exists (duplicate), skipping creation: %s", adminEmail)
					} else {
						log.Printf("Warning: could not create admin user: %v", saveErr)
					}
				} else {
					log.Printf("Admin user created: %s", adminEmail)
				}
			}
		} else {
			desiredUsername := usernameFromEmail(adminEmail)
			if user.GetBool("isAdmin") != true ||
				user.GetString("username") != desiredUsername ||
				user.GetBool("verified") != true {
				user.Set("username", desiredUsername)
				user.Set("isAdmin", true)
				user.Set("role", "admin")
				user.Set("verified", true)
				if updateErr := dao.SaveRecord(user); updateErr != nil {
					log.Printf("Warning: could not repair admin user: %v", updateErr)
				} else {
					log.Printf("Admin user repaired and verified: %s", adminEmail)
				}
			} else {
				log.Printf("Admin user already exists, isAdmin=true, and is verified: %s", adminEmail)
			}
		}
	}

	return nil
}

// hasField checks whether a collection schema contains a field by name.
func hasField(collection *models.Collection, name string) bool {
	for _, f := range collection.Schema.Fields() {
		if f.Name == name {
			return true
		}
	}
	return false
}

// usernameFromEmail derives a valid PocketBase username from an email address.
func usernameFromEmail(email string) string {
	localPart := strings.SplitN(email, "@", 2)[0]
	var b strings.Builder
	for _, r := range localPart {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}

	username := b.String()
	if username == "" {
		return "user"
	}

	first := []rune(username)[0]
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		return "user_" + username
	}
	return username
}
