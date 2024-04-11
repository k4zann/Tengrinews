# Tengrinews

## Overview
Tengrinews is a web application that displays news articles fetched from an external API. Users can browse articles by category, view details of individual articles.

## Features
- Display news in the main page
- Categorize articles for easy navigation and showing latest news in that category, that fetched from the newdata.io
- View detailed information for each article, except for the news that are showed in the category pages, because newsdata.io api, shows the content, and other necessary values only for paid subscribers

## Technologies Used
- Go (Golang) for backend development
- Gorilla Mux for routing
- MongoDB for data storage
- HTML/CSS/JavaScript for frontend UI
- External API for fetching news data (Newsdata.io)


## Setup
1. Clone the repository: `https://github.com/k4zann/Tengrinews.git`
2. Run the code: `go run cmd/main.go`

## Usage
- Access the web application in your browser at `http://localhost:8080`
- Browse news articles by category, view article details

## Need to Improve
- The method for fetching data with using api is already written, but the handler for searching is not ready
- Write my own parser, and schedule it, so that it will parse every morning for the latest news
- Because of using the api, which parses some news not correctly, in the content section the information's written not correctly, with a lot of unnecessary information in it.
- Set the Docker
