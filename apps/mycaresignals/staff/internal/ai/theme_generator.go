package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"therapeutic-engine/internal/models"
)

//
// ────────────────────────────────────────────────────────────────────────────────
//   DATA STRUCTURES
// ────────────────────────────────────────────────────────────────────────────────
//

type DomainThemes struct {
	KeyIssues         []string `json:"key_issues"`
	ProtectiveFactors []string `json:"protective_factors"`
	CurrentResponses  string   `json:"current_responses"`
}

type ThemesResult struct {
	Domains struct {
		SafetyAndRisk           DomainThemes `json:"safety_and_risk"`
		EmotionalAndMental      DomainThemes `json:"emotional_and_mental_health"`
		RelationshipsAttachment DomainThemes `json:"relationships_and_attachment"`
		EducationAndLearning    DomainThemes `json:"education_and_learning"`
		IdentityAndBelonging    DomainThemes `json:"identity_and_belonging"`
		HealthAndSelfCare       DomainThemes `json:"health_and_self_care"`
		IndependenceLifeSkills  DomainThemes `json:"independence_and_life_skills"`
	} `json:"domains"`
	Tags struct {
		SafetyRiskTags []string `json:"safety_risk_tags"`
		EmotionalTags  []string `json:"emotional_tags"`
		PracticalTags  []string `json:"practical_tags"`
	} `json:"tags"`
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   MAIN FUNCTION — AI THEMES EXTRACTION
// ────────────────────────────────────────────────────────────────────────────────
//

// GenerateTherapeuticThemes extracts structured themes from documents using OpenAI.
func GenerateTherapeuticThemes(
	ctx context.Context,
	child *models.Child,
	docs []SourceDocument,
	source DocumentSource,
) (*ThemesResult, error) {
	// 1) Validate input
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents provided for theme extraction")
	}

	// 2) Load API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(apiKey)

	// 3) Build prompts
	docContext := buildDocsContext(docs)
	systemPrompt := buildThemesSystemPrompt()
	userPrompt := buildThemesUserPrompt(child, docs, source, docContext)

	// 4) Build request
	req := openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
	}

	// 5) Call OpenAI
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai themes error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices for themes")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// 6) Parse JSON
	var result ThemesResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("themes: model returned invalid JSON: %w\ncontent=%s", err, content)
	}

	return &result, nil
}

//
// ────────────────────────────────────────────────────────────────────────────────
//   PROMPT BUILDERS
// ────────────────────────────────────────────────────────────────────────────────
//

// System prompt: Stable role + instructions
func buildThemesSystemPrompt() string {
	return `
You are an experienced UK children's residential practitioner.

You understand:
- Children’s Homes (England) Regulations 2015 and the Quality Standards.
- What Ofsted inspectors and Regulation 44 visitors look for in care planning and risk management.
- Trauma-informed, strengths-based, child-centred practice.

Your task NOW is NOT to write a full profile.
Your task is ONLY to extract structured THEMES so that staff and therapists
can later prepare a full Therapeutic Profile.

Always:
- Be specific but concise.
- Include strengths and protective factors.
- Avoid clinical jargon unless explained in plain English.`
}

// User prompt: Child + document context + schema instructions
func buildThemesUserPrompt(
	child *models.Child,
	docs []SourceDocument,
	source DocumentSource,
	docContext string,
) string {
	namePart := child.Initials

	dob := formatField(child.DateOfBirth)
	legalStatus := formatField(child.LegalStatus)
	gender := formatField(child.Gender)

	var sourceDescription string
	if source == DocumentSourceExisting {
		sourceDescription = "This is for a child already placed in the home. Documents represent current plans and risk assessments."
	} else {
		sourceDescription = "This is for a new referral / soon-to-be-admitted child. Documents represent the LA referral pack and associated reports."
	}

	return fmt.Sprintf(`
You are extracting THEMES ONLY for a child in a UK children's home.

Child (pseudonymised):
- Initials/Name: %s
- Date of birth: %s
- Legal status: %s
- Gender: %s

Context:
- %s

You have multiple documents (care/placement plan, ICSP, risk assessment, MFH plan, referral, reports).
Below is a truncated combined excerpt. Use only what you see. Do not invent anything.

---------------- DOCUMENT CONTEXT (TRUNCATED) ----------------
%s
----------------------------------------------------------------

TASK:
Return a JSON object with this EXACT structure:

{
  "domains": {
    "safety_and_risk": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "emotional_and_mental_health": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "relationships_and_attachment": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "education_and_learning": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "identity_and_belonging": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "health_and_self_care": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    },
    "independence_and_life_skills": {
      "key_issues": string[],
      "protective_factors": string[],
      "current_responses": string
    }
  },
  "tags": {
    "safety_risk_tags": string[],
    "emotional_tags": string[],
    "practical_tags": string[]
  }
}

DETAILED GUIDANCE:

- "key_issues": concise bullet-style points showing main concerns.
- "protective_factors": strengths or positives (skills, interests, relationships).
- "current_responses": short description of how the home or network is responding now.

Tags:
- safety_risk_tags: ["self_harm", "suicidal_ideation", "mfh_pattern", "cse_concerns", "cce_concerns", "violence", "weapons", "substance_use", "online_risk", "fire_setting"]
- emotional_tags: ["trauma", "anxiety", "anger", "low_mood", "emotional_dysregulation", "attachment_difficulties", "asd", "adhd"]
- practical_tags: ["education_gap", "contact_complex", "placement_instability", "independent_living", "health_needs"]

RULES:
- If information is missing, say so ("current_responses": "Not clear from documents").
- DO NOT return commentary outside the JSON.
- DO NOT add markdown, only the JSON object.
`,
		namePart,
		dob,
		legalStatus,
		gender,
		sourceDescription,
		docContext,
	)
}
