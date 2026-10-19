#!/usr/bin/env bash
set -e

API_URL="${API_URL:-http://localhost:8080}"

echo "Seeding feedback into $API_URL"
echo

# 1) Happy parent
echo "1) Parent – very positive"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Parent",
    "reason": "Visit review",
    "rating": 5,
    "what_went_well": "The home felt very clean and welcoming. Staff were friendly and made us feel comfortable.",
    "what_to_improve": "Maybe a few more games or activities available in the lounge.",
    "recommend": true,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 2) Parent – positive with small suggestion
echo "2) Parent – positive with suggestion"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Parent",
    "reason": "Contact visit",
    "rating": 4,
    "what_went_well": "Staff were warm and patient with my child. They clearly explained how contact works.",
    "what_to_improve": "Sometimes the visit room felt a bit noisy when other families were using it.",
    "recommend": true,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 3) Social worker – balanced
echo "3) SocialWorker – balanced"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "SocialWorker",
    "reason": "LAC review",
    "rating": 3,
    "what_went_well": "Keyworkers knew the young person well and could describe daily routines and progress.",
    "what_to_improve": "Recording on the system could be more up to date before meetings.",
    "recommend": true,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 4) Ofsted – general safe visit
echo "4) Ofsted – general visit"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Ofsted",
    "reason": "General visit",
    "rating": 4,
    "what_went_well": "Staff interactions with young people looked caring and relaxed. The atmosphere was calm.",
    "what_to_improve": "Young people said they would like more structured activities at weekends.",
    "recommend": true,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 5) Professional – safe, mild issue
echo "5) Professional – safe, mild issue"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Professional",
    "reason": "Therapy contact",
    "rating": 3,
    "what_went_well": "Staff provided useful background and were open to psychological advice.",
    "what_to_improve": "Sometimes handovers to therapy felt rushed because staff were very busy.",
    "recommend": true,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 6) Professional – bullying => MEDIUM risk
echo "6) Professional – bullying concern (MEDIUM)"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Professional",
    "reason": "School feedback",
    "rating": 2,
    "what_went_well": "Staff responded quickly when the school raised concerns.",
    "what_to_improve": "One young person reported being bullied by another child in the home and said they feel unsafe in the lounge at times.",
    "recommend": false,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 7) Parent – threats => MEDIUM risk
echo "7) Parent – threats and feeling scared (MEDIUM)"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Parent",
    "reason": "Phone call after incident",
    "rating": 2,
    "what_went_well": "Staff called me to explain what happened and tried to reassure me.",
    "what_to_improve": "My child said another young person threatened them and they felt scared going back to the bedroom.",
    "recommend": false,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 8) Parent – self-harm => HIGH risk
echo "8) Parent – self harm disclosure (HIGH)"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Parent",
    "reason": "Emotional wellbeing",
    "rating": 1,
    "what_went_well": "Staff listened when my child became upset.",
    "what_to_improve": "My child told me they wanted to self-harm and felt that no one understood how serious it was.",
    "recommend": false,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 9) Social worker – abuse/hit => HIGH risk
echo "9) SocialWorker – physical abuse disclosure (HIGH)"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "SocialWorker",
    "reason": "Safeguarding concern",
    "rating": 1,
    "what_went_well": "Staff have been available to talk through the concerns.",
    "what_to_improve": "A young person disclosed that another child hit them several times and they feel they are being abused in the home.",
    "recommend": false,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo

# 10) Professional – suicide thoughts => HIGH risk
echo "10) Professional – suicide thoughts (HIGH)"
curl -sS -X POST "$API_URL/api/v1/feedback" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "Millbrow House – Care4EveryChild – QR Feedback",
    "who": "Professional",
    "reason": "CAMHS contact",
    "rating": 1,
    "what_went_well": "The home engaged with CAMHS and supported the appointment.",
    "what_to_improve": "The young person said they sometimes want to kill myself and think about suicide. This should be closely monitored and reviewed.",
    "recommend": false,
    "contact_name": "",
    "contact_email": ""
  }'
echo
echo "Done."
