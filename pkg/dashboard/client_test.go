/*
 Copyright 2026 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package dashboard

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEnsureErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1", ensureErrorResponse)
	group.GET("/render-error", func(c *gin.Context) {
		// NaN cannot be encoded as JSON, so gin aborts without writing a body.
		c.IndentedJSON(http.StatusOK, math.NaN())
	})
	group.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/render-error", nil))
	if w.Code != http.StatusInternalServerError || w.Body.Len() == 0 {
		t.Fatalf("expected 500 with error body, got %d %q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ok", nil))
	if w.Code != http.StatusOK || w.Body.String() != `{"ok":true}` {
		t.Fatalf("unexpected response %d %q", w.Code, w.Body.String())
	}
}
