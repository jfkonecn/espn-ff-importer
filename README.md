# ESPN Fantasy Football Importer

This project provides tools to download and analyze ESPN Fantasy Football league data.

## Features

- **Data Download**: Download ESPN fantasy football league data using the `get-season-results.sh` script
- **Static Website Generation**: Generate a beautiful, responsive website with Tailwind CSS styling
- **Custom Standings**: Calculate standings using head-to-head wins and top-half scoring points
- **Payout Tracking**: Track weekly high scorers ($10) and final standings payouts ($200/$100/$50)
- **Recent Games**: Display all games with most recent first

## Prerequisites

- Go 1.21 or later
- Bash shell
- ESPN fantasy football league credentials (ESPN_LEAGUE_ID, ESPN_ESPNS2, ESPN_SWID)

## Setup

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd espn-ff-importer
   ```

2. Set up your ESPN credentials as environment variables:

   ```bash
   export ESPN_LEAGUE_ID="your_league_id"
   export ESPN_ESPNS2="your_espns2_token"
   export ESPN_SWID="your_swid_token"
   ```

3. Initialize the Go module:
   ```bash
   go mod tidy
   ```

## Usage

### Downloading Data

To download league data for a specific year:

```bash
./get-season-results.sh 2024
```

This will create a JSON file in the `data/` directory with the format `espn_league_YYYY.json`.

### Generating the Website

Use the `run-analyzer.sh` script to generate a static HTML website:

```bash
./run-analyzer.sh data/espn_league_2024.json
```

This will create an `index.html` file that you can open in any web browser.

#### Custom Output File

You can specify a custom output file name:

```bash
./run-analyzer.sh data/espn_league_2024.json league.html
```

#### Website Features

The generated website includes:

- **Standings Table**: Shows teams ranked by custom scoring system
  - Head-to-head wins (1 point each)
  - Top-half scoring weeks (1 point each)
  - Total points for tiebreaking
- **Payouts Section**:
  - Weekly high scorers ($10 each)
  - Final standings payouts ($200/$100/$50 for top 3)
- **Recent Games**: All matchups sorted by most recent first
- **Responsive Design**: Works on desktop and mobile devices

## Project Structure

```
espn-ff-importer/
├── data/                    # Downloaded JSON data files
├── src/                     # Go source code
│   ├── types.go            # Data type definitions
│   ├── reader.go           # JSON file reader and data access functions
│   ├── website.go          # Static website generator
│   ├── main.go             # Website generation interface
│   └── templates/          # HTML templates
│       └── website.html    # Main website template
├── get-season-results.sh   # Script to download ESPN data
├── run-analyzer.sh         # Script to generate the website
├── go.mod                  # Go module definition
└── README.md              # This file
```

## Data Types

The Go code includes comprehensive type definitions for ESPN fantasy football data:

- **ESPNLeague**: Main league structure
- **Team**: Fantasy team information
- **Member**: League member details
- **Matchup**: Game matchups between teams
- **Record**: Team win/loss records
- **TransactionCounter**: Team transaction statistics
- **Standing**: Custom standings with head-to-head and top-half scoring
- **Payout**: Weekly high scorers and final standings payouts
- **TemplateData**: Data structure for HTML template rendering
- **StandingRow/GameRow/etc.**: Template-specific data structures

## Getting ESPN Credentials

To get your ESPN credentials:

1. Log into ESPN Fantasy Football
2. Open your browser's developer tools
3. Go to the Network tab
4. Navigate to your league page
5. Look for API requests and extract the cookies:
   - `espn_s2` value
   - `SWID` value
6. Find your league ID from the URL or API requests

## Voice

```sh
ffmpeg -ss 00:00:30 -i input.mp4 -t 10 -vn -acodec pcm_s16le -ar 44100 -ac 2 output.wav
```

[Chatterbox TTS Server](https://github.com/devnen/Chatterbox-TTS-Server) works
the best locally.
