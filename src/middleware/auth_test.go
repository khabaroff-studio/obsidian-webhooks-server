package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

// Test helpers to reduce duplication

// testValidateKeySuccess tests successful key validation
func testValidateKeySuccess(t *testing.T, tdb *database.TestDB, paramName string, middlewareFunc gin.HandlerFunc, getKey func() (string, error)) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)
	router.Use(middlewareFunc)
	router.GET("/test/:"+paramName, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	key, err := getKey()
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}

	c.Request = httptest.NewRequest(http.MethodGet, "/test/"+key, nil)
	router.ServeHTTP(w, c.Request)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// testValidateKeyInvalid tests validation with invalid key
func testValidateKeyInvalid(t *testing.T, tdb *database.TestDB, paramName string, invalidKey string, middlewareFunc gin.HandlerFunc) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)
	router.Use(middlewareFunc)
	router.GET("/test/:"+paramName, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test/"+invalidKey, nil)
	router.ServeHTTP(w, c.Request)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid key, got %d", w.Code)
	}
}

// testAuthMiddlewareInvalidToken tests middleware with invalid token
func testAuthMiddlewareInvalidToken(t *testing.T, middlewareFunc gin.HandlerFunc) {
	gin.SetMode(gin.TestMode)

	// Initialize JWT secret
	originalSecret := JWTSecret
	if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
		t.Fatalf("SetJWTSecret failed: %v", err)
	}
	defer func() { JWTSecret = originalSecret }()

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)
	router.Use(middlewareFunc)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid_token")
	router.ServeHTTP(w, c.Request)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGenerateUserToken(t *testing.T) {
	// Initialize JWT secret for test (must be at least 32 characters)
	originalSecret := JWTSecret
	if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
		t.Fatalf("SetJWTSecret failed: %v", err)
	}
	defer func() {
		JWTSecret = originalSecret
	}()

	webhookKeyID := uuid.New().String()
	token, err := GenerateUserToken(webhookKeyID)
	if err != nil {
		t.Fatalf("GenerateUserToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	// Parse and validate
	claims, err := ValidateUserToken(token)
	if err != nil {
		t.Fatalf("ValidateUserToken failed: %v", err)
	}

	if claims.WebhookKeyID != webhookKeyID {
		t.Errorf("Expected webhook_key_id %s, got %s", webhookKeyID, claims.WebhookKeyID)
	}
}

func TestValidateWebhookKey_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		db := database.NewDatabaseFromPool(tdb.Pool)
		keyService := services.NewKeyService(db.GetPool())
		middleware := ValidateWebhookKey(keyService)

		testValidateKeySuccess(t, tdb, "webhook_key", middleware, func() (string, error) {
			_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
			return webhookKey, err
		})
	})
}

func TestValidateWebhookKey_MissingKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		middleware := ValidateWebhookKey(keyService)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test/", nil)
		// No webhook_key param

		middleware(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

// TODO: Fix bug - ErrKeyNotFound should return 401, not 500
func TestValidateWebhookKey_InvalidKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		db := database.NewDatabaseFromPool(tdb.Pool)
		keyService := services.NewKeyService(db.GetPool())
		middleware := ValidateWebhookKey(keyService)

		testValidateKeyInvalid(t, tdb, "webhook_key", "wh_invalid", middleware)
	})
}

func TestValidateClientKey_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		db := database.NewDatabaseFromPool(tdb.Pool)
		keyService := services.NewKeyService(db.GetPool())
		middleware := ValidateClientKey(keyService)

		testValidateKeySuccess(t, tdb, "client_key", middleware, func() (string, error) {
			_, _, _, clientKey, err := tdb.CreateTestKeyPair(123456, "testuser")
			return clientKey, err
		})
	})
}

// TODO: Fix bug - ErrKeyNotFound should return 401, not 500
func TestValidateClientKey_InvalidKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		db := database.NewDatabaseFromPool(tdb.Pool)
		keyService := services.NewKeyService(db.GetPool())
		middleware := ValidateClientKey(keyService)

		testValidateKeyInvalid(t, tdb, "client_key", "ck_invalid", middleware)
	})
}

func TestAdminAuthMiddleware_WithValidCookie(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		// Initialize JWT secret
		originalSecret := JWTSecret
		if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
			t.Fatalf("SetJWTSecret failed: %v", err)
		}
		defer func() { JWTSecret = originalSecret }()

		// Generate admin token
		adminID := uuid.New()
		token, err := GenerateAdminToken(adminID, "testadmin")
		if err != nil {
			t.Fatalf("GenerateAdminToken failed: %v", err)
		}

		middleware := AdminAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			adminID, _ := c.Get("admin_id")
			username, _ := c.Get("username")
			c.JSON(http.StatusOK, gin.H{
				"admin_id": adminID,
				"username": username,
			})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.AddCookie(&http.Cookie{Name: "admin_token", Value: token})
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestAdminAuthMiddleware_WithValidHeader(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		// Initialize JWT secret
		originalSecret := JWTSecret
		if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
			t.Fatalf("SetJWTSecret failed: %v", err)
		}
		defer func() { JWTSecret = originalSecret }()

		// Generate admin token
		adminID := uuid.New()
		token, err := GenerateAdminToken(adminID, "testadmin")
		if err != nil {
			t.Fatalf("GenerateAdminToken failed: %v", err)
		}

		middleware := AdminAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestAdminAuthMiddleware_MissingToken(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := AdminAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestAdminAuthMiddleware_InvalidToken(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		middleware := AdminAuthMiddleware()
		testAuthMiddlewareInvalidToken(t, middleware)
	})
}

func TestUserAuthMiddleware_WithValidCookie(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		// Initialize JWT secret
		originalSecret := JWTSecret
		if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
			t.Fatalf("SetJWTSecret failed: %v", err)
		}
		defer func() { JWTSecret = originalSecret }()

		// Generate user token
		webhookKeyID := uuid.New().String()
		token, err := GenerateUserToken(webhookKeyID)
		if err != nil {
			t.Fatalf("GenerateUserToken failed: %v", err)
		}

		middleware := UserAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			wkID, _ := c.Get("webhook_key_id")
			c.JSON(http.StatusOK, gin.H{"webhook_key_id": wkID})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.AddCookie(&http.Cookie{Name: "user_token", Value: token})
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestUserAuthMiddleware_WithValidHeader(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		// Initialize JWT secret
		originalSecret := JWTSecret
		if err := SetJWTSecret("test-secret-for-unit-tests-32ch!"); err != nil {
			t.Fatalf("SetJWTSecret failed: %v", err)
		}
		defer func() { JWTSecret = originalSecret }()

		// Generate user token
		webhookKeyID := uuid.New().String()
		token, err := GenerateUserToken(webhookKeyID)
		if err != nil {
			t.Fatalf("GenerateUserToken failed: %v", err)
		}

		middleware := UserAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestUserAuthMiddleware_MissingToken(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := UserAuthMiddleware()

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestUserAuthMiddleware_InvalidToken(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		middleware := UserAuthMiddleware()
		testAuthMiddlewareInvalidToken(t, middleware)
	})
}

func TestValidateBearerToken_ValidToken(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := ValidateBearerToken("test-secret")

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			token, _ := c.Get("token")
			c.JSON(http.StatusOK, gin.H{"token": token})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.Header.Set("Authorization", "Bearer mytoken123")
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

func TestValidateBearerToken_MissingHeader(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := ValidateBearerToken("test-secret")

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func TestValidateBearerToken_InvalidFormat(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)

		middleware := ValidateBearerToken("test-secret")

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Request.Header.Set("Authorization", "InvalidFormat")
		router.ServeHTTP(w, c.Request)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
