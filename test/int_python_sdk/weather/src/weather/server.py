from typing import Any
import asyncio
import httpx
from mcp.server.models import InitializationOptions, ServerCapabilities
from mcp.types import ToolsCapability
import mcp.types as types
from mcp.server import NotificationOptions, Server
import mcp.server.stdio

NWS_API_BASE = "https://api.weather.gov"
USER_AGENT = "weather-app/1.0"

server = Server("weather")

@server.list_tools()
async def handle_list_tools() -> list[types.Tool]:
    """
    List available tools.
    Each tool specifies its parameters using JSON Schema validation.
    """
    return [
        types.Tool(
            name="get-alerts",
            description="Get weather alerts for a state",
            inputSchema={
                "type": "object",
                "properties": {
                    "state": {
                        "type": "string",
                        "description": "Two-letter state code (e.g. CA, NY)",
                    },
                },
                "required": ["state"],
            },
        ),
        types.Tool(
            name="get-forecast",
            description="Get weather forecast for a location",
            inputSchema={
                "type": "object",
                "properties": {
                    "latitude": {
                        "type": "number",
                        "description": "Latitude of the location",
                    },
                    "longitude": {
                        "type": "number",
                        "description": "Longitude of the location",
                    },
                },
                "required": ["latitude", "longitude"],
            },
        ),
    ]

async def make_nws_request(client: httpx.AsyncClient, url: str) -> dict | None:
    """
    Make a request to the NWS API with proper headers and error handling.
    """
    try:
        response = await client.get(url, headers={"User-Agent": USER_AGENT})
        response.raise_for_status()
        return response.json()
    except (httpx.HTTPError, ValueError) as e:
        print(f"Error making request to {url}: {e}")
        return None

def format_alert(alert: dict) -> str:
    """
    Format an alert into a concise string.
    """
    properties = alert.get("properties", {})
    return f"{properties.get('event', 'Unknown Event')} - {properties.get('headline', 'No details available')}"

@server.call_tool()
async def handle_call_tool(
    name: str, arguments: dict | None
) -> list[types.TextContent | types.ImageContent | types.EmbeddedResource]:
    """
    Handle tool execution requests.
    Tools can fetch weather data and notify clients of changes.
    """
    if not arguments:
        raise ValueError("Missing arguments")
  
    if name == "get-alerts":
        state = arguments.get("state")
        if not state:
            raise ValueError("Missing state parameter")

        # Convert state to uppercase to ensure consistent format
        state = state.upper()
        if len(state) != 2:
            raise ValueError("State must be a two-letter code (e.g. CA, NY)")

        async with httpx.AsyncClient() as client:
            alerts_url = f"{NWS_API_BASE}/alerts?area={state}"
            alerts_data = await make_nws_request(client, alerts_url)

            if not alerts_data:
                return [types.TextContent(type="text", text="Failed to retrieve alerts data")]

            features = alerts_data.get("features", [])
            if not features:
                return [types.TextContent(type="text", text=f"No active alerts for {state}")]

            # Format each alert into a concise string
            formatted_alerts = [format_alert(feature) for feature in features[:20]] # only take the first 20 alerts
            alerts_text = f"Active alerts for {state}:\n\n" + "\n".join(formatted_alerts)

            return [
                types.TextContent(
                    type="text",
                    text=alerts_text
                )
            ]
    elif name == "get-forecast":
        try:
            latitude = float(arguments.get("latitude"))
            longitude = float(arguments.get("longitude"))
        except (TypeError, ValueError):
            return [types.TextContent(
                type="text",
                text="Invalid coordinates. Please provide valid numbers for latitude and longitude."
            )]
            
        # Basic coordinate validation
        if not (-90 <= latitude <= 90) or not (-180 <= longitude <= 180):
            return [types.TextContent(
                type="text",
                text="Invalid coordinates. Latitude must be between -90 and 90, longitude between -180 and 180."
            )]

        async with httpx.AsyncClient() as client:
            # First get the grid point
            lat_str = f"{latitude}"
            lon_str = f"{longitude}"
            points_url = f"{NWS_API_BASE}/points/{lat_str},{lon_str}"
            points_data = await make_nws_request(client, points_url)

            if not points_data:
                return [types.TextContent(type="text", text=f"Failed to retrieve grid point data for coordinates: {latitude}, {longitude}. This location may not be supported by the NWS API (only US locations are supported).")]

            # Get the forecast URL from the points response
            forecast_url = points_data.get("properties", {}).get("forecast")
            if not forecast_url:
                return [types.TextContent(type="text", text="No forecast available for this location")]

            # Get the forecast data
            forecast_data = await make_nws_request(client, forecast_url)
            if not forecast_data:
                return [types.TextContent(type="text", text="Failed to retrieve forecast data")]

            # Format the forecast periods
            periods = forecast_data.get("properties", {}).get("periods", [])
            if not periods:
                return [types.TextContent(type="text", text="No forecast periods available")]

            # Format the forecast into a readable string
            forecast_text = "Weather Forecast:\n\n"
            for period in periods[:10]:  # Only show next 10 periods
                forecast_text += f"{period.get('name', 'Unknown')}:\n"
                forecast_text += f"Temperature: {period.get('temperature', 'N/A')}°{period.get('temperatureUnit', 'F')}\n"
                forecast_text += f"Conditions: {period.get('shortForecast', 'N/A')}\n"
                forecast_text += f"Details: {period.get('detailedForecast', 'N/A')}\n"
                forecast_text += "---\n"

            return [types.TextContent(type="text", text=forecast_text)]

    else:
        raise ValueError(f"Unknown tool: {name}")

async def main():
    # Run the server using stdin/stdout streams
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            InitializationOptions(
                server_name="weather",
                server_version="0.1.0",
                capabilities=ServerCapabilities(
                    tools=ToolsCapability(listChanged=False)
                )
            ),
        )

# This is needed if you'd like to connect to a custom client
if __name__ == "__main__":
    asyncio.run(main())
