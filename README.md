# Ticker Analyzer

A simple website written in Go that performs Risk Analysis and DCF for a given ticker.

This is mainly a side project for me to learn the Go programming language, and also experiment with some of the financial concepts that I have been learning as part of my CFA prep, and create a tool that I could ultimately use to analyze stocks of interest.

## Financial Data Sources

→ **Alpaca-Market** is used for historic price data


→ **FMP** is the primary data source for a ticker’s Income statement and balance sheet data. Currently the free tier is being used, so data is only limited to top companies such as Apple, Nvidia. etc


→ **Yahoo Finance** is the backup source if FMP does not provide the ticker information. Since Yahoo blocks web               scrapers, I used yFinance, a popular python library that is used to access yahoo’s data. To do this, a python                 FastAPI microservice is also being run

## Risk Analysis Metrics

* **99VaR:** Measures the 99th percentile worst case Scenario. We calculate this by running a Monte Carlo simulation on 100,000 random scenarios, with the simulation parameters calculated from historic price data
* **Beta**: How correlated the ticker is to the general stock market.
* **Monte Carlo Simulation visualization**

## Discount CashFlow Analysis

* DCF is a valuation methodology where you make educated projections regarding a company’s future revenue growths, operating margins, etc, and discount the future projected cashflows back to today’s value to get an estimate of the company’s Intrinsic value.
* The calculation methodoly used in this tool is based on the Aswath Damodaran’s metholody as explained in his book “Narrative and Numbers”
* We make use of the company’s historic Income Statements and Balance Sheet data to make educated guesses about the company’s DCF input values
* The DCF table is interacitve, so feel free to change the input values and see how the company’s value changes

## Tech Stack

* Go - http module
* Python - FastAPI
* Frontend - HTMX, TailwindCSS

## How to Run

### 1. Environment Variables

Before running the application, create a `.env` file in the root directory of the project and add your required API keys.

```env
FMP_API_KEY=your_financial_modeling_prep_key
ALPACA_API_KEY=your_alpaca_markets_key
```

### 2. Python Microservice setup

```python
cd FinanceDataPy

# Create and activate a virtual environment

python -m venv venv
source venv/bin/activate  # On Windows use: venv\Scripts\activate

# Install dependencies

pip install -r requirements.txt

# Start the FastAPI server
uvicorn main:app --port 8000
```

### 3. Go Backend

```python
go mod tidy

// Start the server
go run main.go
```


