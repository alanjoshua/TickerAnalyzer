from fastapi import FastAPI, HTTPException
import yfinance as yf
import json

app = FastAPI()

@app.get("/api/fundamentals/{ticker}")
async def get_fundamentals(ticker: str):
    try:
        stock = yf.Ticker(ticker, )
        info = stock.info

        # Make a copy and then drop the last column (data from 5 years ago is null since yfinance free version does not support it)
        income = stock.income_stmt.copy().iloc[:, :4]
        balance = stock.balance_sheet.copy().iloc[:, :4]
        cash = stock.cashflow.copy().iloc[:, :4]

        # TEMPORARY measure specifically for Nebius, since it is a stock of interest for me, and I know that because they went from yandex to Nebius,
        #  their financial numbers only make sense after 2022
        # TODO: Find a better way to handle special case situations like this, maybe from the UI the user could specify the number of years of historic data to use
        if ticker == "NBIS":
            income = income.iloc[:, :3]
            balance = balance.iloc[:, :3]
            cash = stock.cashflow.iloc[:, :3]

        # For the sake of parity with the FMP api, we move Shares outstanding from the balance sheet to the income statement
        if "Share Issued" in balance.index:
            income.loc["Shares Outstanding"] = balance.loc["Share Issued"]
        else:
            income.loc["Shares Outstanding"] = float('nan')

        # Convert from dict with the date as key, to an ordered array, where ind=0 should contain the latest data
        income = income.T 
        income = income.reset_index().rename(columns={"index": "date"})

        balance = balance.T
        balance = balance.reset_index().rename(columns={"index": "date"})
        
        cash = cash.T
        cash = cash.reset_index().rename(columns={"index": "date"})

        income_stmt = json.loads(income.to_json(orient="records", date_format="iso"))
        balance_sheet = json.loads(balance.to_json(orient="records", date_format="iso"))
        cashflow = json.loads(cash.to_json(orient="records", date_format="iso"))

        return {
            "symbol": ticker,
            "currentPrice": info.get("currentPrice", 0),
            "marketCap": info.get("marketCap", 0),
            "beta": info.get("beta", 1.0),
            "incomeStatement": income_stmt,
            "balanceSheet": balance_sheet,
            "cashflow": cashflow
        }

    except Exception as e:
        return HTTPException(status_code=500, detail=str(e))