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
URL="https://lm-api-reads.fantasy.espn.com/apis/v3/games/ffl/seasons/$YEAR/segments/0/leagues/$ESPN_LEAGUE_ID?view=mMatchupScore&view=mScoreboard&view=mTeam&view=mSettings"

# Build the Cookie header
COOKIE="espn_s2=$ESPN_ESPNS2; SWID=$ESPN_SWID"

# Download the data into the data directory
curl -sSL \
  -H "Cookie: $COOKIE" \
  "$URL" -o "data/espn_league_${YEAR}.json"

