#!/usr/bin/env bash
set -e

API_URL="${API_URL:-http://localhost:8080}"

echo "Seeding young people's monthly feedback into $API_URL"
echo

# All examples use Millbrow House and November 2025.
# period_month follows your frontend pattern: "YYYY-MM"

# 1) Young person A – mostly positive month
echo "1) Young person – mostly positive month"
curl -sS -X POST "$API_URL/api/v1/young-voice" \
  -H "Content-Type: application/json" \
  -d '{
    "home": "Millbrow House",
    "young_person_alias": "A.",
    "period_month": "2025-11",

    "q1_overall": "This month has been mostly alright. I feel more settled than before.",
    "q1_best": "Going to the cinema and having a takeaway night with staff.",
    "q1_hardest": "Missing my family some evenings.",

    "q2_safe": "I feel safe in my bedroom and when I am with staff in the lounge.",
    "q2_unsafe": "",
    "q2_support_who": "My key worker and one night staff I trust.",

    "q3_listened": "When I asked to change my bedtime slightly, staff listened and we agreed a plan.",
    "q3_trusted_adult": "My key worker because they always explain things properly.",
    "q3_change_support": "Sometimes I want staff to explain rules more calmly when I am upset.",

    "q4_peers": "I get on with one of the other young people and we play games together.",
    "q4_unkind": "",
    "q4_help_with_peers": "",

    "q5_enjoy": "Going out for walks, playing Xbox and cooking in the kitchen.",
    "q5_wish_more": "More planned activities at weekends.",
    "q5_house_change": "Maybe make the garden nicer so we can sit outside.",

    "q6_education": "School has been okay. I have been going more regularly this month.",
    "q6_education_help": "Reminders in the morning really help me get ready on time.",

    "q7_contact_feelings": "I like my phone calls home and they make me feel less homesick.",
    "q7_contact_change": "Sometimes I would like a bit longer on the phone.",

    "q8_say_in_decisions": "I feel like I get a bit of a say in my plans.",
    "q8_change_example": "They let me pick which activities I wanted on my birthday.",

    "q9_worries": "I still worry that things might change suddenly.",
    "q9_one_change": "Have more notice when plans change.",
    "q9_want_talk": false,
    "q9_follow_up_preference": "",

    "q10_what_tell_ofsted": "The home is mostly good and staff try to help even when I kick off.",
    "q10_never_forget": "I have been through a lot before coming here so please remember that."
  }'
echo
echo

# 2) Young person B – bullying / feeling unsafe (MEDIUM concern)
echo "2) Young person – bullying / feeling unsafe (MEDIUM)"
curl -sS -X POST "$API_URL/api/v1/young-voice" \
  -H "Content-Type: application/json" \
  -d '{
    "home": "Millbrow House",
    "young_person_alias": "B.",
    "period_month": "2025-11",

    "q1_overall": "This month has been hard because of arguments in the house.",
    "q1_best": "Going to town with staff and having some time away from the house.",
    "q1_hardest": "People shouting at night and not being able to sleep.",

    "q2_safe": "I feel safest when I am downstairs with staff or out of the home.",
    "q2_unsafe": "Sometimes I feel unsafe in the lounge when another young person shouts and threatens me.",
    "q2_support_who": "I would go to my key worker or the senior on shift.",

    "q3_listened": "Staff did listen when I said I was scared but I feel like nothing really changed.",
    "q3_trusted_adult": "One of the night staff because they check in with me.",
    "q3_change_support": "Take it more seriously when I say I am scared of another young person.",

    "q4_peers": "There is one young person who is okay but another one is really unkind to me.",
    "q4_unkind": "I have been called names and laughed at which makes me not want to come out of my room.",
    "q4_help_with_peers": "I would like staff to help us sort it properly and not just tell us to ignore each other.",

    "q5_enjoy": "Listening to music in my room and going for drives.",
    "q5_wish_more": "More 1:1 time out of the home.",
    "q5_house_change": "Less shouting and proper consequences when people are mean.",

    "q6_education": "I am not going to school much because I feel too anxious.",
    "q6_education_help": "Support with getting a smaller timetable would help.",

    "q7_contact_feelings": "Sometimes contact is good, sometimes it leaves me feeling upset afterwards.",
    "q7_contact_change": "More support after difficult calls.",

    "q8_say_in_decisions": "I feel like decisions are mostly made for me.",
    "q8_change_example": "",

    "q9_worries": "I worry that I will get hurt if arguments carry on.",
    "q9_one_change": "Make the house feel calmer and safer.",
    "q9_want_talk": true,
    "q9_follow_up_preference": "I want to talk to my key worker and a manager together.",

    "q10_what_tell_ofsted": "Sometimes the home is okay but there is too much shouting and I do not always feel safe.",
    "q10_never_forget": "I am trying my best even when I look like I do not care."
  }'
echo
echo

# 3) Young person C – mixed, wants more say, some risk words
echo "3) Young person – mixed, low/medium concern"
curl -sS -X POST "$API_URL/api/v1/young-voice" \
  -H "Content-Type: application/json" \
  -d '{
    "home": "Millbrow House",
    "young_person_alias": "C.",
    "period_month": "2025-11",

    "q1_overall": "Up and down. Some good days, some really low days.",
    "q1_best": "Staff helping me apply for college.",
    "q1_hardest": "Feeling like nobody really understands how low I get sometimes.",

    "q2_safe": "I feel okay in my room when I can just shut the door.",
    "q2_unsafe": "",
    "q2_support_who": "One staff member that I trust, not everyone.",

    "q3_listened": "Sometimes they listen but sometimes they just say it is the rules.",
    "q3_trusted_adult": "My key worker.",
    "q3_change_support": "Check on me more when I look quiet rather than only when I am angry.",

    "q4_peers": "I mostly stay to myself. I do not like big groups.",
    "q4_unkind": "",
    "q4_help_with_peers": "Smaller activities where everyone is calmer.",

    "q5_enjoy": "Art, drawing and going for coffee.",
    "q5_wish_more": "More chances to do things that are for my age not just childish stuff.",
    "q5_house_change": "Make my room feel more like mine.",

    "q6_education": "I am waiting to start college. I feel worried it will not work out.",
    "q6_education_help": "Support with travel training and meeting tutors.",

    "q7_contact_feelings": "Contact is complicated and sometimes it makes my mood drop.",
    "q7_contact_change": "More say in when and how contact happens.",

    "q8_say_in_decisions": "I want more of a say in big decisions like where I live next.",
    "q8_change_example": "",

    "q9_worries": "I sometimes think about hurting myself when I feel really low but I have not told many people.",
    "q9_one_change": "More emotional support and someone I can talk to properly.",
    "q9_want_talk": true,
    "q9_follow_up_preference": "I would like to talk to my key worker and maybe a therapist.",

    "q10_what_tell_ofsted": "The home tries but they do not always see how much I am struggling inside.",
    "q10_never_forget": "I am more than just my behaviour and I want a future."
  }'
echo
echo

# 4) Young person D – high concern (self-harm / suicide)
echo "4) Young person – self-harm / suicide thoughts (HIGH)"
curl -sS -X POST "$API_URL/api/v1/young-voice" \
  -H "Content-Type: application/json" \
  -d '{
    "home": "Millbrow House",
    "young_person_alias": "D.",
    "period_month": "2025-11",

    "q1_overall": "This month has been really bad. I have felt depressed most days.",
    "q1_best": "One evening when staff sat and watched a film with me.",
    "q1_hardest": "Feeling like I do not want to be here anymore.",

    "q2_safe": "I only feel safe when a staff member I trust is nearby.",
    "q2_unsafe": "I do not feel safe when I am left alone for long because I think about self-harm.",
    "q2_support_who": "One key worker I trust and sometimes the manager.",

    "q3_listened": "Sometimes I say I want to hurt myself and I feel like people just think I am attention seeking.",
    "q3_trusted_adult": "My key worker.",
    "q3_change_support": "Take it seriously when I say I want to hurt myself and check on me more.",

    "q4_peers": "I do not really get on with anyone here.",
    "q4_unkind": "People have said horrible things about me and it makes me feel worse.",
    "q4_help_with_peers": "Separate me from people who wind me up when I am already low.",

    "q5_enjoy": "Hard to think of anything. Maybe listening to music.",
    "q5_wish_more": "I wish we could go out more instead of just staying in.",
    "q5_house_change": "Make my room feel safer and calmer.",

    "q6_education": "I am barely going to education because my mood is too low.",
    "q6_education_help": "Support with getting mental health help before pushing school too much.",

    "q7_contact_feelings": "Contact sometimes makes me feel like I am a problem.",
    "q7_contact_change": "More support before and after contact.",

    "q8_say_in_decisions": "I feel like I have no control over my life.",
    "q8_change_example": "",

    "q9_worries": "I think about killing myself sometimes and feel like nobody would care.",
    "q9_one_change": "I want real help with my mental health, not just being told to calm down.",
    "q9_want_talk": true,
    "q9_follow_up_preference": "I need to speak to my key worker and a mental health professional.",

    "q10_what_tell_ofsted": "Sometimes staff try but the help is not enough for how bad I feel.",
    "q10_never_forget": "I am really struggling and I need people to notice."
  }'
echo
echo "Done seeding young people's monthly feedback."
