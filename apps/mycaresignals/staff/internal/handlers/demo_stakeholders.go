package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"

	"therapeutic-engine/internal/models"
	"therapeutic-engine/internal/security"
	"therapeutic-engine/internal/storage"
)

const defaultDemoHomeID int64 = 1

type CognitoStakeholderInviter interface {
	AdminAddUserToGroup(context.Context, *cognitoidentityprovider.AdminAddUserToGroupInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminAddUserToGroupOutput, error)
	AdminCreateUser(context.Context, *cognitoidentityprovider.AdminCreateUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error)
	AdminGetUser(context.Context, *cognitoidentityprovider.AdminGetUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminGetUserOutput, error)
	AdminListGroupsForUser(context.Context, *cognitoidentityprovider.AdminListGroupsForUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminListGroupsForUserOutput, error)
}

type DemoStakeholderHandler struct {
	Repo        *storage.Repo
	Cognito     CognitoStakeholderInviter
	UserPoolID  string
	DemoHomeID  int64
	Environment string
}

func NewDemoStakeholderHandler(repo *storage.Repo, cognito CognitoStakeholderInviter) *DemoStakeholderHandler {
	return &DemoStakeholderHandler{
		Repo:        repo,
		Cognito:     cognito,
		UserPoolID:  strings.TrimSpace(os.Getenv("COGNITO_USER_POOL_ID")),
		DemoHomeID:  demoHomeIDFromEnv(),
		Environment: strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("APP_ENVIRONMENT"), os.Getenv("APP_ENV")))),
	}
}

func (h *DemoStakeholderHandler) Access(w http.ResponseWriter, r *http.Request) {
	if !h.demoEnabled() {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("demo stakeholder handler not initialised"))
		return
	}
	canInvite, err := h.canInvite(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	canSubmitFeedback, err := h.canSubmitFeedback(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	canViewFeedback, err := h.canViewFeedback(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"demo_enabled":        true,
		"is_admin":            canInvite && canViewFeedback,
		"can_invite":          canInvite,
		"can_submit_feedback": canSubmitFeedback,
		"can_view_feedback":   canViewFeedback,
	})
}

func (h *DemoStakeholderHandler) Invite(w http.ResponseWriter, r *http.Request) {
	h.handleInvite(w, r, false)
}

func (h *DemoStakeholderHandler) ResendInvite(w http.ResponseWriter, r *http.Request) {
	h.handleInvite(w, r, true)
}

func (h *DemoStakeholderHandler) SubmitProductFeedback(w http.ResponseWriter, r *http.Request) {
	if !h.demoEnabled() {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("demo stakeholder feedback is not configured"))
		return
	}
	canSubmit, err := h.canSubmitFeedback(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if !canSubmit {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	principal, ok := security.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		OverallRating      int    `json:"overall_rating"`
		MostUseful         string `json:"most_useful"`
		UnclearOrDifficult string `json:"unclear_or_difficult"`
		Missing            string `json:"missing"`
		AnythingElse       string `json:"anything_else"`
		PagePath           string `json:"page_path"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	if payload.OverallRating < 1 || payload.OverallRating > 5 {
		writeBadRequest(w, "overall rating must be between 1 and 5")
		return
	}
	item, err := h.Repo.CreateStakeholderProductFeedback(r.Context(), models.StakeholderProductFeedbackInput{
		ReviewerStaffUserID: principal.StaffUserID(),
		ReviewerName:        principal.DisplayName(),
		ReviewerEmail:       principal.Email(),
		Organisation:        "",
		Environment:         "demo",
		PagePath:            payload.PagePath,
		OverallRating:       payload.OverallRating,
		MostUseful:          payload.MostUseful,
		UnclearOrDifficult:  payload.UnclearOrDifficult,
		Missing:             payload.Missing,
		AnythingElse:        payload.AnythingElse,
	})
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *DemoStakeholderHandler) ListProductFeedback(w http.ResponseWriter, r *http.Request) {
	if !h.demoEnabled() {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Repo == nil {
		writeServerError(w, errors.New("demo stakeholder feedback is not configured"))
		return
	}
	canView, err := h.canViewFeedback(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if !canView {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	items, err := h.Repo.ListStakeholderProductFeedback(r.Context(), 100)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *DemoStakeholderHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	if !h.demoEnabled() {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Repo == nil || h.Cognito == nil || strings.TrimSpace(h.UserPoolID) == "" {
		writeServerError(w, errors.New("demo stakeholder invite is not configured"))
		return
	}
	canInvite, err := h.canInvite(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if !canInvite {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	invites, err := h.Repo.ListDemoStakeholders(r.Context(), h.DemoHomeID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	for index := range invites {
		status, err := h.safeCognitoStatus(r.Context(), invites[index].Email)
		if err == nil && status != "" {
			invites[index].Status = status
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": invites})
}

func (h *DemoStakeholderHandler) handleInvite(w http.ResponseWriter, r *http.Request, resend bool) {
	if !h.demoEnabled() {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Repo == nil || h.Cognito == nil || strings.TrimSpace(h.UserPoolID) == "" {
		writeServerError(w, errors.New("demo stakeholder invite is not configured"))
		return
	}
	canInvite, err := h.canInvite(r.Context())
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if !canInvite {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var payload struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Organisation string `json:"organisation"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	email, err := normalizeInviteEmail(payload.Email)
	if err != nil {
		writeBadRequest(w, "enter a valid email address")
		return
	}
	name := normalizeInviteName(payload.Name)
	if name == "" {
		writeBadRequest(w, "enter a stakeholder name")
		return
	}

	user, alreadyInvited, err := h.ensureCognitoUser(r.Context(), email, name, resend)
	if err != nil {
		writeServerError(w, err)
		return
	}
	sub := cognitoAttribute(user.UserAttributes, "sub")
	if sub == "" {
		writeServerError(w, errors.New("demo stakeholder Cognito sub missing"))
		return
	}
	if _, err := h.Repo.UpsertDemoStakeholder(r.Context(), email, name, sub, h.DemoHomeID); err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":             name,
		"email":            email,
		"organisation":     strings.TrimSpace(payload.Organisation),
		"status":           inviteStatus(alreadyInvited),
		"already_invited":  alreadyInvited,
		"personal_message": personalStakeholderMessage(name),
	})
}

func (h *DemoStakeholderHandler) canInvite(ctx context.Context) (bool, error) {
	_, err := security.RequireHomeRole(ctx, h.Repo, h.DemoHomeID, security.RoleAdmin)
	if err != nil {
		if errors.Is(err, security.ErrForbidden) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (h *DemoStakeholderHandler) canSubmitFeedback(ctx context.Context) (bool, error) {
	_, err := security.RequireHomeRole(ctx, h.Repo, h.DemoHomeID, security.RoleResponsibleIndividual, security.RoleDemoViewer, security.RoleAdmin)
	if err != nil {
		if errors.Is(err, security.ErrForbidden) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (h *DemoStakeholderHandler) canViewFeedback(ctx context.Context) (bool, error) {
	_, err := security.RequireHomeRole(ctx, h.Repo, h.DemoHomeID, security.RoleAdmin)
	if err != nil {
		if errors.Is(err, security.ErrForbidden) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrUnauthenticated):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, security.ErrForbidden), errors.Is(err, security.ErrInvalidRelationship):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		writeServerError(w, err)
	}
}

func (h *DemoStakeholderHandler) ensureCognitoUser(ctx context.Context, email, name string, resend bool) (*cognitoidentityprovider.AdminGetUserOutput, bool, error) {
	user, err := h.Cognito.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(h.UserPoolID),
		Username:   aws.String(email),
	})
	alreadyInvited := true
	if err != nil {
		var notFound *types.UserNotFoundException
		if !errors.As(err, &notFound) {
			return nil, false, err
		}
		alreadyInvited = false
		if _, err := h.Cognito.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
			UserPoolId: aws.String(h.UserPoolID),
			Username:   aws.String(email),
			UserAttributes: []types.AttributeType{
				{Name: aws.String("email"), Value: aws.String(email)},
				{Name: aws.String("email_verified"), Value: aws.String("true")},
				{Name: aws.String("name"), Value: aws.String(name)},
			},
			DesiredDeliveryMediums: []types.DeliveryMediumType{types.DeliveryMediumTypeEmail},
		}); err != nil {
			return nil, false, err
		}
		user, err = h.Cognito.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
			UserPoolId: aws.String(h.UserPoolID),
			Username:   aws.String(email),
		})
		if err != nil {
			return nil, false, err
		}
	} else if resend {
		if _, err := h.Cognito.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
			UserPoolId: aws.String(h.UserPoolID),
			Username:   aws.String(email),
			UserAttributes: []types.AttributeType{
				{Name: aws.String("email"), Value: aws.String(email)},
				{Name: aws.String("email_verified"), Value: aws.String("true")},
				{Name: aws.String("name"), Value: aws.String(name)},
			},
			DesiredDeliveryMediums: []types.DeliveryMediumType{types.DeliveryMediumTypeEmail},
			MessageAction:          types.MessageActionTypeResend,
		}); err != nil {
			return nil, false, err
		}
	}
	if _, err := h.Cognito.AdminAddUserToGroup(ctx, &cognitoidentityprovider.AdminAddUserToGroupInput{
		UserPoolId: aws.String(h.UserPoolID),
		Username:   aws.String(email),
		GroupName:  aws.String(string(security.RoleDemoViewer)),
	}); err != nil {
		return nil, false, err
	}
	return user, alreadyInvited, nil
}

func (h *DemoStakeholderHandler) safeCognitoStatus(ctx context.Context, email string) (string, error) {
	user, err := h.Cognito.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(h.UserPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return "", err
	}
	groups, err := h.Cognito.AdminListGroupsForUser(ctx, &cognitoidentityprovider.AdminListGroupsForUserInput{
		UserPoolId: aws.String(h.UserPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return "", err
	}
	hasDemoViewer := false
	for _, group := range groups.Groups {
		if aws.ToString(group.GroupName) == string(security.RoleDemoViewer) {
			hasDemoViewer = true
			break
		}
	}
	if !hasDemoViewer {
		return "Invited", nil
	}
	switch user.UserStatus {
	case types.UserStatusTypeConfirmed:
		return "Active", nil
	case types.UserStatusTypeForceChangePassword, types.UserStatusTypeResetRequired:
		return "Account setup pending", nil
	default:
		return "Invited", nil
	}
}

func (h *DemoStakeholderHandler) demoEnabled() bool {
	return h != nil && strings.EqualFold(h.Environment, "demo")
}

func normalizeInviteEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, " ") || strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return "", errors.New("invalid email")
	}
	at := strings.LastIndex(value, "@")
	if at <= 0 || at != strings.Index(value, "@") || at >= len(value)-3 {
		return "", errors.New("invalid email")
	}
	domain := value[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", errors.New("invalid email")
	}
	return value, nil
}

func normalizeInviteName(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func cognitoAttribute(attributes []types.AttributeType, name string) string {
	for _, attribute := range attributes {
		if strings.EqualFold(aws.ToString(attribute.Name), name) {
			return strings.TrimSpace(aws.ToString(attribute.Value))
		}
	}
	return ""
}

func personalStakeholderMessage(name string) string {
	firstName := name
	if fields := strings.Fields(name); len(fields) > 0 {
		firstName = fields[0]
	}
	return "Hi " + firstName + " — I've just sent you an invitation to the myCareSignals stakeholder demo. " +
		"You should receive an email with a secure link to set up your access. " +
		"I'd really value your thoughts once you've had a chance to look around. Thanks, Victory"
}

func demoHomeIDFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv("DEMO_HOME_ID"))
	if raw == "" {
		return defaultDemoHomeID
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return defaultDemoHomeID
	}
	return id
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func inviteStatus(alreadyInvited bool) string {
	if alreadyInvited {
		return "already_invited"
	}
	return "sent"
}
