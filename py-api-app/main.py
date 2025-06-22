from fastapi import FastAPI, Query
from pydantic import BaseModel
from typing import List, Optional
from sqlalchemy import create_engine, text
from fastapi import HTTPException
import pandas as pd
from math import exp
import numpy as np
from datetime import datetime, timedelta
import os



BASE_DIR = os.path.dirname(os.path.abspath(__file__))  
DB_PATH = os.path.abspath(os.path.join(BASE_DIR, "..", "data_stream/db", "stream_data.db"))  
DATABASE_URL = f"sqlite:///{DB_PATH}"


engine = create_engine(DATABASE_URL, connect_args={"check_same_thread": False})
app = FastAPI()

class Report(BaseModel):
    id: int
    timestamp: str
    stream_id: str
    observations_timestamp: int
    benchmark_price: str
    bid: str
    ask: str
    valid_from_timestamp: int
    expires_at: int
    link_fee: str
    native_fee: str
    synthetic_oi: float

@app.get("/streamrecords", response_model=List[Report])
def get_reports(stream_id: Optional[str] = Query(None), limit: int = 500):
    query = "SELECT * FROM reports"
    params = {}

    if stream_id:
        query += " WHERE stream_id = :stream_id"
        params["stream_id"] = stream_id

    query += " ORDER BY id DESC LIMIT :limit"
    params["limit"] = limit

    with engine.connect() as conn:
        result = conn.execute(text(query), params).mappings().all()
        return result

def scaled_sigmoid(x):
    return 1 / (1 + exp(-x))

@app.get("/reflexivity")
def calculate_reflexivity(stream_id: str):
    with engine.connect() as conn:
        query = """
        SELECT timestamp, benchmark_price, synthetic_oi 
        FROM reports 
        WHERE stream_id = :stream_id
        ORDER BY timestamp DESC
        """
        result = conn.execute(text(query), {"stream_id": stream_id}).mappings().all()

    if not result or len(result) < 30:
        raise HTTPException(status_code=404, detail="Not enough data for computation")

    # Convert to DataFrame
    df = pd.DataFrame(result)
    df['timestamp'] = pd.to_datetime(df['timestamp'])
    df = df.sort_values('timestamp')

    # Convert benchmark_price to float with scaling (assume 18 decimals)
    df['benchmark_price'] = df['benchmark_price'].astype(float) / 1e18
    df['synthetic_oi'] = df['synthetic_oi'].astype(float) / 1e20

    # Compute EMA
    df['ema_5'] = df['benchmark_price'].ewm(span=5, adjust=False).mean()
    df['ema_10'] = df['benchmark_price'].ewm(span=10, adjust=False).mean()

    m = (df.iloc[-1]['ema_5'] - df.iloc[-1]['ema_10']) / df.iloc[-1]['ema_10']

    # Compute volatility v
    last_30_prices = df['benchmark_price'].tail(30)
    v = last_30_prices.std() / last_30_prices.mean()

    # Compute oig
    now_ts = df.iloc[-1]['timestamp']
    past_30min_df = df[df['timestamp'] <= now_ts - pd.Timedelta(minutes=30)]

    if past_30min_df.empty:
        raise HTTPException(status_code=400, detail="No data from 30 minutes ago")

    synthetic_oi_now = df['synthetic_oi'].iloc[-1]
    synthetic_oi_30min = past_30min_df['synthetic_oi'].iloc[-1]

    if synthetic_oi_30min == 0:
        raise HTTPException(status_code=400, detail="Synthetic OI 30 min ago is zero, division error")

    oig = (synthetic_oi_now - synthetic_oi_30min) / synthetic_oi_30min

    # Weights
    w1 = w2 = w3 = 0.33

    # Reflexivity score before sigmoid
    raw_r = w1 * m + w2 * v + w3 * oig

    # Apply scaled sigmoid to keep score in 0–3 range
    r = scaled_sigmoid(raw_r)

    return {
        "reflexivity_score": round(r, 6),
        "stream_id": stream_id,
        "m": round(m, 6),
        "v": round(v, 6),
        "oig": round(oig, 6),
        "raw_r": round(raw_r, 6)
    }
