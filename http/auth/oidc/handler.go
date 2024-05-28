package oidc

import (
	stdctx "context"
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"

	"github.com/dioad/net/http/auth/context"
)

type Handler struct {
	CookieStore             sessions.Store
	LoginPath               string
	LogoutPath              string
	CallbackDefaultRedirect string
	HomePath                string
}

type SessionData struct {
	ID        uuid.UUID
	Principal string
	Provider  string
	User      goth.User
}

func init() {
	gob.Register(SessionData{})
	gob.Register(uuid.UUID{})
	gob.Register(goth.User{})
}

func (h *Handler) AuthRequest(r *http.Request) (stdctx.Context, error) {
	session, err := h.CookieStore.Get(r, SessionCookieName)
	if err != nil {
		return r.Context(), err
	}

	data := session.Values["data"].(SessionData)

	ctx := context.NewContextWithAuthenticatedPrincipal(r.Context(), data.Principal)
	ctx = NewContextWithOIDCUserInfo(ctx, &data.User)

	return ctx, nil
}

func (h *Handler) handleAuth(w http.ResponseWriter, req *http.Request) (*SessionData, error) {
	session, err := h.CookieStore.Get(req, SessionCookieName)
	if err != nil {
		return nil, err
	}

	if session.IsNew {
		r, _ := h.CookieStore.New(req, PreAuthRefererCookieName)
		r.Values["referer"] = req.URL.String()
		err = h.CookieStore.Save(req, w, r)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}

	data := session.Values["data"].(SessionData)
	return &data, nil
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return h.AuthWrapper(next.ServeHTTP)
}

func (h *Handler) AuthWrapper(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sessionData, err := h.handleAuth(w, req)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if sessionData == nil {
			w.Header().Set("Location", h.LoginPath)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}

		ctx := context.NewContextWithAuthenticatedPrincipal(req.Context(), sessionData.Principal)
		ctx = NewContextWithOIDCUserInfo(ctx, &sessionData.User)

		next.ServeHTTP(w, req.WithContext(ctx))
	}
}

func (h *Handler) AuthStart() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		gothic.BeginAuthHandler(w, req)
	}
}

func (h *Handler) handleCallback(w http.ResponseWriter, req *http.Request) (string, error) {
	user, err := gothic.CompleteUserAuth(w, req)
	if err != nil {
		return "", err
	}

	session, err := h.CookieStore.New(req, SessionCookieName)
	if err != nil {
		return "", err
	}
	params := mux.Vars(req)
	provider := params["provider"]

	user.RawData = nil

	session.Values["data"] = SessionData{
		ID:        uuid.New(),
		User:      user,
		Principal: user.NickName,
		Provider:  provider,
	}

	err = h.CookieStore.Save(req, w, session)
	if err != nil {
		return "", err
	}

	redirect := h.HomePath
	r, _ := h.CookieStore.Get(req, PreAuthRefererCookieName)
	if r.Values["referer"] != nil {
		redirect = r.Values["referer"].(string)
		r.Options.MaxAge = -1
		err = h.CookieStore.Save(req, w, r)
		if err != nil {
			return "", err
		}
	}

	return redirect, nil
}

func (h *Handler) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		redirect, err := h.handleCallback(w, req)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Location", redirect)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func (*Handler) handleLogout(w http.ResponseWriter, req *http.Request) error {
	session, err := gothic.Store.Get(req, SessionCookieName)
	if err != nil {
		return err
	}
	dataValue, ok := session.Values["data"]
	if !ok {
		return fmt.Errorf("no session data found")
	}
	data := dataValue.(SessionData)

	session.Options.MaxAge = -1
	err = gothic.Store.Save(req, w, session)
	if err != nil {
		return err
	}

	// Github Auth has no logout
	if data.Provider != "github" {
		err = gothic.Logout(w, req)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		err := h.handleLogout(w, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Location", h.LoginPath)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func NewHandler(config Config, store sessions.Store) *Handler {
	gothic.Store = store

	provider := config.ProviderMap["github"]

	goth.UseProviders(
		github.New(provider.ClientID, provider.ClientSecret, provider.Callback, "read:user", "user:email"),
	)

	return &Handler{
		CookieStore: store,
	}
}
