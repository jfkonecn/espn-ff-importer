#!/bin/bash

# Exit on error, undefined variable, or pipe failure
set -euo pipefail

# Ensure year is passed
if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <year>"
  exit 1
fi

YEAR="$1"

# Ensure required environment variables are set
: "${ESPN_LEAGUE_ID:?Missing ESPN_LEAGUE_ID}"
: "${ESPN_ESPNS2:?Missing ESPN_ESPNS2}"
: "${ESPN_SWID:?Missing ESPN_SWID}"

# Change to the directory where this script resides
cd "$(dirname "$0")"

# Create the data directory if it doesn't exist
mkdir -p data

# Construct the URL
# URL="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR/segments/0/leagues/$ESPN_LEAGUE_ID?scoringPeriodId=2&view=modular&view=mNav&view=mMatchupScore&view=mScoreboard&view=mSettings&view=mTopPerformers&view=mTeam"
URL_SEASON="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR/segments/0/leagues/$ESPN_LEAGUE_ID?view=mMatchupScore&view=mScoreboard&view=mTeam&view=mSettings&view=mDraftDetail&view=mRoster"

# Build the Cookie header
COOKIE="espn_s2=$ESPN_ESPNS2; SWID=$ESPN_SWID"

# Download the data into the data directory
curl -sSL \
  -H "Cookie: $COOKIE" \
  "$URL_SEASON" -o "data/espn_league_${YEAR}.json"


URL_PLAYERS="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR/players?scoringPeriodId=0&view=players_wl"

# Download the data into the data directory
curl -sSL \
  -H "X-Fantasy-Filter: {\"players\":{\"limit\":10000}}" \
  "$URL_PLAYERS" -o "data/espn_players_${YEAR}.json"


URL_PRO_TEAMS="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR?view=proTeamSchedules_wl"

# Download the data into the data directory
curl -sSL \
  "$URL_PRO_TEAMS" -o "data/espn_pro_teams_${YEAR}.json"


URL_PLAYER_INFO="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR/segments/0/leagues/$ESPN_LEAGUE_ID?view=kona_player_info"

# Download the data into the data directory
curl -sSL \
    -H "Cookie: $COOKIE" \
    -H 'x-fantasy-filter: {"players":{"filterSlotIds":null,"sortPercChanged":{"sortPriority":1,"sortAsc":true},"limit":25,"filterRanksForSlotIds":{"value":[0,2,4,6,17,16,8,9,10,12,13,24,11,14,15]},"filterStatsForTopScoringPeriodIds":{"value":2,"additionalValue":["002022","102022","002021","022022"]}}}' \
  "$URL_PLAYER_INFO" -o "data/espn_most_dropped_${YEAR}.json"

curl -sSL \
    -H "Cookie: $COOKIE" \
    -H 'x-fantasy-filter: {"players":{"filterSlotIds":null,"sortPercChanged":{"sortPriority":1,"sortAsc":false},"limit":25,"filterRanksForSlotIds":{"value":[0,2,4,6,17,16,8,9,10,12,13,24,11,14,15]},"filterStatsForTopScoringPeriodIds":{"value":2,"additionalValue":["002022","102022","002021","022022"]}}}' \
  "$URL_PLAYER_INFO" -o "data/espn_most_added_${YEAR}.json"
