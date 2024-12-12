# Robots.txt API Service

## Overview

The api is designed to manage scrape permissions and create custom rules for specific domains. 
It provides endpoints to check scrape permissions, and to `get`, `create`, `update`, and `delete` custom rules.

## Endpoints

### Health Check

- **GET** `/ping` - Check if the server is running.

### Scrape Permissions

The base URL for the API call is determined by the `RobotsUrlPath` configuration setting.

- **GET** `/scrape-allowed` - Check if scraping is allowed for a given domain by checking the `robots.txt` file.

### Custom Rules

The base URL for the API calls is determined by the `RobotsUrlPath` configuration setting.

- **GET** `/custom-rule` - Retrieve custom rules for a domain.
- **POST** `/custom-rule` - Create a new custom rule.
- **PUT** `/custom-rule` - Update an existing custom rule.
- **DELETE** `/custom-rule` - Delete a custom rule.

### Swagger Documentation

- **GET** `/swagger/index.html` - Access the Swagger UI for API documentation.

## CORS

The API supports Cross-Origin Resource Sharing (CORS) with the following settings:

- **Allowed Methods**: `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`
- **Allowed Headers**: `Content-Type`, `Content-Length`, `Accept-Encoding`, `Authorization`, `X-Forwarded-For`,
  `X-CSRF-Token`, `X-Max`
- **Allow Credentials**: `true`
- **Max Age**: Configurable via `CorsMaxAgeHours`

## Configuration

Configuration file variables can be overridden via global variables.
For example, the value below
<pre>database:
  port: "3306"</pre>
can be overridden by setting the `DATABASE.PORT=3307` environment variable.
