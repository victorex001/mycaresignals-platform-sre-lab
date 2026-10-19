package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

var errInvalidChildOnboardingPayload = errors.New("invalid child onboarding payload")

type ChildrenHandler struct {
	Children *storage.ChildrenStore
	Repo     *storage.Repo
}

type OnboardChildRequest struct {
	ExternalRef   string  `json:"external_ref"`
	Initials      string  `json:"initials"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	DateOfBirth   string  `json:"date_of_birth"` // YYYY-MM-DD
	LegalStatus   string  `json:"legal_status"`
	AdmissionDate *string `json:"admission_date"` // optional, YYYY-MM-DD
	Gender        string  `json:"gender"`
}

// POST /v1/homes/{homeID}/children/onboard
func (h *ChildrenHandler) OnboardChild(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Repo == nil {
		http.Error(w, "repository not initialised", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	homeIDStr := chi.URLParam(r, "homeID")
	homeIDInt, err := strconv.ParseInt(homeIDStr, 10, 64)
	if err != nil || homeIDInt <= 0 {
		http.Error(w, "invalid home id", http.StatusBadRequest)
		return
	}

	var req OnboardChildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	child := &models.Child{HomeID: models.HomeID(homeIDInt), Active: true}
	err = h.Repo.WithTx(ctx, nil, func(tx *storage.TxRepo) error {
		principal, err := security.RequireHomeRoleForMutation(
			ctx,
			tx,
			homeIDInt,
			security.RoleSenior,
			security.RoleManager,
		)
		if err != nil {
			return err
		}
		homeExists, err := tx.LockHomeID(ctx, homeIDInt)
		if err != nil {
			return err
		}
		if !homeExists {
			return security.ErrForbidden
		}

		initials := strings.TrimSpace(req.Initials)
		if initials == "" {
			return fmt.Errorf("%w: initials", errInvalidChildOnboardingPayload)
		}
		dob, err := time.Parse("2006-01-02", req.DateOfBirth)
		if err != nil {
			return fmt.Errorf("%w: date_of_birth", errInvalidChildOnboardingPayload)
		}
		var admissionDate *time.Time
		if req.AdmissionDate != nil && *req.AdmissionDate != "" {
			parsed, err := time.Parse("2006-01-02", *req.AdmissionDate)
			if err != nil {
				return fmt.Errorf("%w: admission_date", errInvalidChildOnboardingPayload)
			}
			admissionDate = &parsed
		}
		child.ExternalRef = strings.TrimSpace(req.ExternalRef)
		child.Initials = initials
		child.FirstName = strings.TrimSpace(req.FirstName)
		child.LastName = strings.TrimSpace(req.LastName)
		child.DateOfBirth = dob
		child.LegalStatus = strings.TrimSpace(req.LegalStatus)
		child.AdmissionDate = admissionDate
		child.Gender = strings.TrimSpace(req.Gender)

		if err := tx.CreateChild(ctx, child); err != nil {
			return err
		}
		bound, err := tx.NewChildAuditEvent(
			ctx,
			principal,
			int64(child.ID),
			storage.AuditEventChildCreate,
			security.AuditMetadataFields{RequestID: chimw.GetReqID(ctx)},
			storage.AuditRouteChildOnboard,
		)
		if err != nil {
			return err
		}
		_, err = tx.InsertAuditEvent(ctx, bound)
		return err
	})
	if err != nil {
		if errors.Is(err, security.ErrUnauthenticated) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, security.ErrForbidden) || errors.Is(err, security.ErrInvalidRelationship) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if errors.Is(err, errInvalidChildOnboardingPayload) {
			http.Error(w, "invalid child payload", http.StatusBadRequest)
			return
		}
		http.Error(w, "could not create child", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(child)
}

// GET /v1/homes/{homeID}/children
// GET /v1/homes/{homeID}/children
func (h *ChildrenHandler) ListChildrenForHome(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Children == nil {
		http.Error(w, "children store not initialised", http.StatusInternalServerError)
		return
	}

	homeIDRaw := chi.URLParam(r, "homeID")
	homeID, err := parseHomeID(homeIDRaw)
	if err != nil {
		writeBadRequest(w, "invalid home id")
		return
	}

	children, err := h.Children.ListChildrenByHome(r.Context(), models.HomeID(homeID))
	if err != nil {
		writeServerError(w, err)
		return
	}

	// ✅ ensure JSON is [] not null
	if children == nil {
		children = make([]*models.Child, 0) // <-- change type if your slice is []*models.Child
	}

	writeJSON(w, http.StatusOK, children)
}
