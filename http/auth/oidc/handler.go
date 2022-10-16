package oidc

import (
	"encoding/gob"
	"net/http"

	"github.com/dioad/net/http/auth/context"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
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

func (h *Handler) AuthWrapper(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		session, err := h.CookieStore.Get(req, SessionCookieName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if session.IsNew {
			r, _ := h.CookieStore.New(req, PreAuthRefererCookieName)
			r.Values["referer"] = req.URL.String()
			h.CookieStore.Save(req, w, r)
			w.Header().Set("Location", h.LoginPath)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}

		data := session.Values["data"].(SessionData)

		ctx := context.NewContextWithAuthenticatedPrincipal(req.Context(), data.Principal)
		ctx = NewContextWithOIDCUserInfo(ctx, &data.User)

		next(w, req.WithContext(ctx))
	}
}

func (h *Handler) AuthStart() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		gothic.BeginAuthHandler(w, req)
	}
}

func (h *Handler) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		user, err := gothic.CompleteUserAuth(w, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		session, err := h.CookieStore.New(req, SessionCookieName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

		h.CookieStore.Save(req, w, session)
		redirect := h.HomePath
		r, _ := h.CookieStore.Get(req, PreAuthRefererCookieName)
		if r.Values["referer"] != nil {
			redirect = r.Values["referer"].(string)
			r.Options.MaxAge = -1
			h.CookieStore.Save(req, w, r)
		}

		w.Header().Set("Location", redirect)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func (h *Handler) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		session, err := h.CookieStore.Get(req, SessionCookieName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := session.Values["data"].(SessionData)

		session.Options.MaxAge = -1
		h.CookieStore.Save(req, w, session)

		// Github Auth has no logout
		if data.Provider != "github" {
			err = gothic.Logout(w, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
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
