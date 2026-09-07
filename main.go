package main

import (
	"log"
	"net/http"
	"os"

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

	// ===== CORS Middleware =====
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
		}))
		return nil
	})

	// ===== Ensure collections AFTER server starts (non-blocking) =====
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		go func() {
			if err := ensureCollections(app); err != nil {
				log.Printf("Error ensuring collections: %v", err)
			}
		}()
		return nil
	})

	// ===== Start the app =====
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func ensureCollections(app *pocketbase.PocketBase) error {
	dao := app.Dao()
	if dao == nil {
		return nil
	}

	// 1. Modify users collection to add isAdmin flag
	usersCollection, err := dao.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	hasIsAdmin := false
	for _, field := range usersCollection.Schema.Fields() {
		if field.Name == "isAdmin" {
			hasIsAdmin = true
			break
		}
	}
	if !hasIsAdmin {
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

	// 2. Create "properties" collection
	if _, err := dao.FindCollectionByNameOrId("properties"); err != nil {
		collection := &models.Collection{
			Name:       "properties",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true"),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true"),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true"),
		}
		fields := []*schema.SchemaField{
			{Name: "title", Type: schema.FieldTypeText, Required: true},
			{Name: "description", Type: schema.FieldTypeText, Required: false},
			{Name: "address", Type: schema.FieldTypeText, Required: true},
			{Name: "price", Type: schema.FieldTypeNumber, Required: true},
			{Name: "type", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values:    []string{"house", "warehouse", "apartment", "villa"},
			}},
			{Name: "listing_type", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values:    []string{"sale", "rent"},
			}},
			{Name: "status", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values:    []string{"available", "sold", "rented"},
			}},
			{Name: "bedrooms", Type: schema.FieldTypeNumber, Required: false},
			{Name: "bathrooms", Type: schema.FieldTypeNumber, Required: false},
			{Name: "size", Type: schema.FieldTypeNumber, Required: false},
			{Name: "latitude", Type: schema.FieldTypeNumber, Required: false},
			{Name: "longitude", Type: schema.FieldTypeNumber, Required: false},
			{Name: "images", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{
				MaxSelect: 10,
				MaxSize:   5242880,
				MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
			}},
			{Name: "video", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{
				MaxSelect: 1,
				MaxSize:   52428800,
				MimeTypes: []string{"video/mp4", "video/webm", "video/quicktime"},
			}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 3. Create "contact_requests" collection
	if _, err := dao.FindCollectionByNameOrId("contact_requests"); err != nil {
		collection := &models.Collection{
			Name:       "contact_requests",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isAdmin = true"),
			ViewRule:   types.Pointer("@request.auth.isAdmin = true"),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true"),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true"),
		}
		fields := []*schema.SchemaField{
			{Name: "property", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{
				CollectionId: "",
				MaxSelect:    types.Pointer(1),
			}},
			{Name: "name", Type: schema.FieldTypeText, Required: true},
			{Name: "email", Type: schema.FieldTypeEmail, Required: false},
			{Name: "phone", Type: schema.FieldTypeText, Required: false},
			{Name: "message", Type: schema.FieldTypeText, Required: true},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		// Set relation to properties collection
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

	// 4. Create "property_requests" collection (scouting)
	if _, err := dao.FindCollectionByNameOrId("property_requests"); err != nil {
		collection := &models.Collection{
			Name:       "property_requests",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isAdmin = true"),
			ViewRule:   types.Pointer("@request.auth.isAdmin = true"),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true"),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true"),
		}
		fields := []*schema.SchemaField{
			{Name: "user", Type: schema.FieldTypeRelation, Required: false, Options: &schema.RelationOptions{
				CollectionId: "",
				MaxSelect:    types.Pointer(1),
			}},
			{Name: "description", Type: schema.FieldTypeText, Required: true},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		// Set user relation
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

	// 5. Create "owner_enquiries" collection
	if _, err := dao.FindCollectionByNameOrId("owner_enquiries"); err != nil {
		collection := &models.Collection{
			Name:       "owner_enquiries",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer("@request.auth.isAdmin = true"),
			ViewRule:   types.Pointer("@request.auth.isAdmin = true"),
			CreateRule: types.Pointer(""),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true"),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true"),
		}
		fields := []*schema.SchemaField{
			{Name: "full_name", Type: schema.FieldTypeText, Required: true},
			{Name: "company_name", Type: schema.FieldTypeText, Required: false},
			{Name: "phone", Type: schema.FieldTypeText, Required: true},
			{Name: "property_address", Type: schema.FieldTypeText, Required: true},
			{Name: "category", Type: schema.FieldTypeSelect, Required: true, Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values:    []string{"general_enquiry", "inspection", "filming"},
			}},
			{Name: "message", Type: schema.FieldTypeText, Required: true},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 6. Create "professionals" collection
	if _, err := dao.FindCollectionByNameOrId("professionals"); err != nil {
		collection := &models.Collection{
			Name:       "professionals",
			Type:       models.CollectionTypeBase,
			ListRule:   types.Pointer(""),
			ViewRule:   types.Pointer(""),
			CreateRule: types.Pointer("@request.auth.isAdmin = true"),
			UpdateRule: types.Pointer("@request.auth.isAdmin = true"),
			DeleteRule: types.Pointer("@request.auth.isAdmin = true"),
		}
		fields := []*schema.SchemaField{
			{Name: "name", Type: schema.FieldTypeText, Required: true},
			{Name: "role", Type: schema.FieldTypeText, Required: true},
			{Name: "company", Type: schema.FieldTypeText, Required: false},
			{Name: "email", Type: schema.FieldTypeEmail, Required: false},
			{Name: "avatar", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{
				MaxSelect: 1,
				MaxSize:   2097152,
				MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
			}},
			{Name: "logo", Type: schema.FieldTypeFile, Required: false, Options: &schema.FileOptions{
				MaxSelect: 1,
				MaxSize:   2097152,
				MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
			}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// 7. Create "favorites" collection
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
			{Name: "user", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{
				CollectionId: "",
				MaxSelect:    types.Pointer(1),
			}},
			{Name: "property", Type: schema.FieldTypeRelation, Required: true, Options: &schema.RelationOptions{
				CollectionId: "",
				MaxSelect:    types.Pointer(1),
			}},
		}
		for _, f := range fields {
			collection.Schema.AddField(f)
		}
		// Set relation IDs
		usersColl, _ := dao.FindCollectionByNameOrId("users")
		propColl, _ := dao.FindCollectionByNameOrId("properties")
		if usersColl != nil && propColl != nil {
			for _, f := range collection.Schema.Fields() {
				if f.Name == "user" {
					f.Options.(*schema.RelationOptions).CollectionId = usersColl.Id
				}
				if f.Name == "property" {
					f.Options.(*schema.RelationOptions).CollectionId = propColl.Id
				}
			}
		}
		// Add unique index
		collection.Indexes = types.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_favorites_user_property ON favorites (user, property)",
		}
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}
	}

	// (Optional) Create a PocketBase superuser if env vars set
	pbAdminEmail := os.Getenv("PB_ADMIN_EMAIL")
	pbAdminPassword := os.Getenv("PB_ADMIN_PASSWORD")
	if pbAdminEmail != "" && pbAdminPassword != "" {
		// Check if superuser exists
		_, err := dao.FindAuthRecordByEmail("_admins", pbAdminEmail)
		if err != nil {
			// Create superuser
			adminCollection, _ := dao.FindCollectionByNameOrId("_admins")
			newAdmin := models.NewRecord(adminCollection)
			newAdmin.Set("email", pbAdminEmail)
			newAdmin.Set("password", pbAdminPassword)
			newAdmin.Set("passwordConfirm", pbAdminPassword)
			if err := dao.SaveRecord(newAdmin); err != nil {
				log.Printf("Warning: could not create PocketBase superuser: %v", err)
			} else {
				log.Printf("PocketBase superuser created: %s", pbAdminEmail)
			}
		}
	}

	// (Optional) Create a regular admin user if env vars set
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminEmail != "" && adminPassword != "" {
		user, err := dao.FindAuthRecordByEmail("users", adminEmail)
		if err != nil {
			// create user
			collection, _ := dao.FindCollectionByNameOrId("users")
			newUser := models.NewRecord(collection)
			newUser.Set("email", adminEmail)
			newUser.Set("username", adminEmail)
			newUser.Set("password", adminPassword)
			newUser.Set("passwordConfirm", adminPassword)
			newUser.Set("isAdmin", true)
			if err := dao.SaveRecord(newUser); err != nil {
				log.Printf("Warning: could not create admin user: %v", err)
			}
		} else {
			if user.GetBool("isAdmin") != true {
				user.Set("isAdmin", true)
				if err := dao.SaveRecord(user); err != nil {
					log.Printf("Warning: could not update admin user: %v", err)
				}
			}
		}
	}

	return nil
}
