// internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"therapeutic-engine/internal/handlers"
	appmw "therapeutic-engine/internal/middleware"
	"therapeutic-engine/internal/security"
)

type Deps struct {
	ChildrenHandler         *handlers.ChildrenHandler
	GoalsHandler            *handlers.GoalsHandler
	RiskHandler             *handlers.RiskHandler
	CarePlanHandler         *handlers.CarePlanHandler
	CareOverviewHandler     *handlers.CareOverviewHandler
	ExternalFeedbackHandler *handlers.ExternalFeedbackHandler
	DailyProgressHandler    *handlers.DailyProgressHandler
	DemoStakeholderHandler  *handlers.DemoStakeholderHandler
	MissingFromHomeHandler  *handlers.MissingFromHomePlanHandler
	LocalDevelopmentAccess  *handlers.LocalDevelopmentAccessHandler
	TherapeuticHandler      *handlers.TherapeuticHandler
	Authenticator           security.Authenticator
	AuthorizationStore      security.AuthorizationStore
	FeedbackIntakeToken     string
}

func New(deps Deps) http.Handler {
	if deps.ChildrenHandler == nil {
		panic("router.New: ChildrenHandler is nil")
	}
	if deps.GoalsHandler == nil {
		panic("router.New: GoalsHandler is nil")
	}
	if deps.RiskHandler == nil {
		panic("router.New: RiskHandler is nil")
	}
	if deps.CarePlanHandler == nil {
		panic("router.New: CarePlanHandler is nil")
	}
	if deps.CareOverviewHandler == nil {
		panic("router.New: CareOverviewHandler is nil")
	}
	if deps.ExternalFeedbackHandler == nil {
		panic("router.New: ExternalFeedbackHandler is nil")
	}
	if deps.DailyProgressHandler == nil {
		panic("router.New: DailyProgressHandler is nil")
	}
	if deps.DemoStakeholderHandler == nil {
		panic("router.New: DemoStakeholderHandler is nil")
	}
	if deps.MissingFromHomeHandler == nil {
		panic("router.New: MissingFromHomeHandler is nil")
	}
	if deps.LocalDevelopmentAccess == nil {
		panic("router.New: LocalDevelopmentAccess is nil")
	}
	if deps.TherapeuticHandler == nil {
		panic("router.New: TherapeuticHandler is nil")
	}
	if deps.Authenticator == nil {
		panic("router.New: Authenticator is nil")
	}
	if deps.AuthorizationStore == nil {
		panic("router.New: AuthorizationStore is nil")
	}

	childReadRoles := []security.Role{
		security.RoleKeyWorker,
		security.RoleSenior,
		security.RoleManager,
		security.RoleResponsibleIndividual,
		security.RoleDemoViewer,
	}

	// -------------------------
	// 1) Build the "app" router ONCE (your real routes)
	// -------------------------
	app := chi.NewRouter()

	// Middlewares
	app.Use(chimw.RequestID)
	app.Use(chimw.Logger)
	app.Use(chimw.Recoverer)
	app.Use(appmw.Cors)

	// Healthcheck
	app.Get("/healthz", handlers.Health)

	// Main API
	app.Route("/v1", func(r chi.Router) {

		// 🔹 Home-level routes
		r.Route("/homes/{homeID}", func(r chi.Router) {
			r.With(
				appmw.Authenticate(deps.Authenticator),
				appmw.AuthorizeHomeRead(deps.AuthorizationStore, childReadRoles...),
			).Get("/children", deps.ChildrenHandler.ListChildrenForHome)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/children/onboard", deps.ChildrenHandler.OnboardChild)

			r.With(appmw.Authenticate(deps.Authenticator)).Post(
				"/children/{childID}/onboarding/baseline",
				deps.ChildrenHandler.SaveOnboardingBaseline,
			)
			r.With(appmw.Authenticate(deps.Authenticator)).Post(
				"/children/{childID}/onboarding/framework",
				deps.ChildrenHandler.SaveOnboardingFramework,
			)

		})

		r.With(appmw.Authenticate(deps.Authenticator)).Get(
			"/demo/stakeholders/access",
			deps.DemoStakeholderHandler.Access,
		)
		r.With(appmw.Authenticate(deps.Authenticator)).Get(
			"/demo/stakeholders/invites",
			deps.DemoStakeholderHandler.ListInvites,
		)
		r.With(appmw.Authenticate(deps.Authenticator)).Post(
			"/demo/stakeholders/invite",
			deps.DemoStakeholderHandler.Invite,
		)
		r.With(appmw.Authenticate(deps.Authenticator)).Post(
			"/demo/stakeholders/invite/resend",
			deps.DemoStakeholderHandler.ResendInvite,
		)
		r.With(appmw.Authenticate(deps.Authenticator)).Post(
			"/demo/stakeholders/product-feedback",
			deps.DemoStakeholderHandler.SubmitProductFeedback,
		)
		r.With(appmw.Authenticate(deps.Authenticator)).Get(
			"/demo/stakeholders/product-feedback",
			deps.DemoStakeholderHandler.ListProductFeedback,
		)

		// 🔹 Child-level routes
		r.Route("/children/{childID}", func(r chi.Router) {
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/missing-from-home-plans", deps.MissingFromHomeHandler.GetWorkspace)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/missing-from-home-plans/reuse-candidates", deps.MissingFromHomeHandler.ListReuseCandidates)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/missing-from-home-plans/{planID}", deps.MissingFromHomeHandler.GetPlan)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/missing-from-home-plans", deps.MissingFromHomeHandler.CreateDraft)
			r.With(appmw.Authenticate(deps.Authenticator)).Put("/missing-from-home-plans/{planID}", deps.MissingFromHomeHandler.SaveDraft)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/missing-from-home-plans/{planID}/submit", deps.MissingFromHomeHandler.Submit)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/missing-from-home-plans/{planID}/approve", deps.MissingFromHomeHandler.Approve)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/missing-from-home-plans/{planID}/revise", deps.MissingFromHomeHandler.Revise)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/missing-from-home-plans/{planID}/archive", deps.MissingFromHomeHandler.Archive)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-overview", deps.CareOverviewHandler.GetChildCareOverview)

			r.With(appmw.Authenticate(deps.Authenticator)).Post("/daily-progress", deps.DailyProgressHandler.Create)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/daily-progress", deps.DailyProgressHandler.List)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/daily-progress/monthly", deps.DailyProgressHandler.Monthly)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/monthly-voice", deps.ExternalFeedbackHandler.ListChildMonthlyVoice)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/monthly-voice/{receiptID}", deps.ExternalFeedbackHandler.GetChildMonthlyVoice)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, security.RoleSenior, security.RoleManager)).Get("/monthly-voice/review-access", deps.ExternalFeedbackHandler.MonthlyVoiceReviewAccess)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, security.RoleSenior, security.RoleManager)).Post("/monthly-voice/{receiptID}/reviews", deps.ExternalFeedbackHandler.ReviewChildMonthlyVoice)

			r.With(appmw.Authenticate(deps.Authenticator)).Post("/goals", deps.GoalsHandler.CreateGoal)
			r.With(appmw.Authenticate(deps.Authenticator)).Patch("/goals/{goalID}", deps.GoalsHandler.UpdateGoal)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/goals", deps.GoalsHandler.ListGoals)

			r.With(appmw.Authenticate(deps.Authenticator)).Post("/targets", deps.GoalsHandler.CreateTarget)
			r.With(appmw.Authenticate(deps.Authenticator)).Patch("/targets/{targetID}", deps.GoalsHandler.UpdateTarget)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/targets", deps.GoalsHandler.ListTargets)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/risks", deps.RiskHandler.ListRisks)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/risks", deps.RiskHandler.CreateRisk)
			r.With(appmw.Authenticate(deps.Authenticator)).Patch("/risks/{riskID}", deps.RiskHandler.UpdateRisk)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/risks/{riskID}/reviews", deps.RiskHandler.ListRiskReviews)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/risks/{riskID}/reviews", deps.RiskHandler.CreateRiskReview)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans", deps.CarePlanHandler.GetCurrentCarePlan)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans/history", deps.CarePlanHandler.ListCarePlans)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans/access", deps.CarePlanHandler.GetCarePlanAccess)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans/{carePlanID}", deps.CarePlanHandler.GetCarePlan)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans/{carePlanID}/reviews", deps.CarePlanHandler.ListCarePlanReviews)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/care-plans/{carePlanID}/review-evidence", deps.CarePlanHandler.GetCarePlanReviewEvidence)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/care-plans", deps.CarePlanHandler.CreateCarePlan)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/care-plans/{carePlanID}/reviews", deps.CarePlanHandler.CreateCarePlanReview)
			r.With(appmw.Authenticate(deps.Authenticator)).Put("/care-plans/{carePlanID}/sections/{sectionKey}", deps.CarePlanHandler.UpdateCarePlanSection)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/care-plans/{carePlanID}/activate", deps.CarePlanHandler.ActivateCarePlan)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/care-plans/{carePlanID}/archive", deps.CarePlanHandler.ArchiveCarePlan)

			r.With(appmw.Authenticate(deps.Authenticator)).Post("/sessions", deps.TherapeuticHandler.CreateSessionWithScores)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/sessions/{sessionID}/manager-review", deps.TherapeuticHandler.CreateManagerReview)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/sessions", deps.TherapeuticHandler.ListSessions)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/timeline", deps.TherapeuticHandler.Timeline)
			 r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/domain_index", deps.TherapeuticHandler.GetChildDomainIndex)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/checkins", deps.TherapeuticHandler.GetDomainTimeline)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/reviews", deps.TherapeuticHandler.ListPsychologicalReviews)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/reviews", deps.TherapeuticHandler.CreatePsychologicalReview)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/signal_responses", deps.ChildrenHandler.ListSignalResponses)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/signal_responses", deps.ChildrenHandler.LogSignalResponse)

			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/onboarding/baseline", deps.ChildrenHandler.GetOnboardingBaseline)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/onboarding/approve", deps.ChildrenHandler.ApproveOnboardingBaseline)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/onboarding/framework", deps.ChildrenHandler.GetOnboardingFramework)
			r.With(appmw.Authenticate(deps.Authenticator), appmw.AuthorizeChildRead(deps.AuthorizationStore, childReadRoles...)).Get("/onboarding/framework/history", deps.ChildrenHandler.ListOnboardingFrameworkHistory)
			r.With(appmw.Authenticate(deps.Authenticator)).Post("/onboarding/framework/approve", deps.ChildrenHandler.ApproveOnboardingFramework)

			// DEMO SEED (REMOVE AFTER DEMO)
			r.Post("/demo_seed", deps.TherapeuticHandler.DemoSeedSessions)
		})
	})

	app.With(appmw.RequireFeedbackIntakeToken(deps.FeedbackIntakeToken)).Post(
		"/internal/v1/feedback-intake/receipts",
		deps.ExternalFeedbackHandler.IngestReceipt,
	)
	app.Post("/internal/dev/local-manager-token", deps.LocalDevelopmentAccess.IssueManagerToken)

	// -------------------------
	// 2) Root router: expose app BOTH at "/" and "/api/te"
	// -------------------------
	root := chi.NewRouter()

	// ALB paths: /api/te/healthz, /api/te/v1/...
	root.Mount("/api/te", http.StripPrefix("/api/te", app))

	root.Mount("/", app)

	return root
}
