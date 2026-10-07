"use client";

import React, { useState } from "react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import { TrendingUp, AlertCircle, Cpu, Activity, ShieldCheck } from "lucide-react";

const mockEquityData = [
  { time: "10:00", equity: 10000 },
  { time: "10:15", equity: 10050 },
  { time: "10:30", equity: 9980 },
  { time: "10:45", equity: 10120 },
  { time: "11:00", equity: 10250 },
  { time: "11:15", equity: 10400 },
];

const mockTrades = [
  { id: "01925b4a-1", symbol: "BTCUSDT", action: "BUY", price: 65120.5, qty: 0.05, pnl: 0, reason: "EMA Golden Cross + Bullish sentiment", time: "10:45:12" },
  { id: "01925b4a-2", symbol: "BTCUSDT", action: "SELL", price: 66200.0, qty: 0.05, pnl: 53.97, reason: "RSI Take-profit threshold reached", time: "11:14:50" },
];

export default function Dashboard() {
  const [trades] = useState(mockTrades);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-8 font-sans">
      <header className="flex justify-between items-center mb-8 border-b border-slate-800 pb-4">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Cpu className="text-emerald-400" />
            Autonomous Crypto Trading Agent
          </h1>
          <p className="text-sm text-slate-400">Binance WebSocket Ingestion • GPT-4o-mini Reasoning • pgvector Memory</p>
        </div>
        <div className="flex gap-4">
          <span className="flex items-center gap-1.5 px-3 py-1 bg-emerald-950 border border-emerald-800 text-emerald-400 rounded-full text-xs font-medium">
            <Activity size={14} className="animate-pulse" /> Live Ingestion
          </span>
          <span className="flex items-center gap-1.5 px-3 py-1 bg-blue-950 border border-blue-800 text-blue-400 rounded-full text-xs font-medium">
            <ShieldCheck size={14} /> Guardrails Active
          </span>
        </div>
      </header>

      {/* Metric Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <p className="text-xs text-slate-400 uppercase font-semibold">Total Portfolio Equity</p>
          <h2 className="text-3xl font-extrabold text-emerald-400 mt-1">$10,400.00</h2>
          <span className="text-xs text-emerald-500 font-medium">+4.00% Net Return</span>
        </div>
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <p className="text-xs text-slate-400 uppercase font-semibold">Win Rate</p>
          <h2 className="text-3xl font-extrabold text-white mt-1">75.8%</h2>
          <span className="text-xs text-slate-500">94 Wins / 30 Losses</span>
        </div>
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <p className="text-xs text-slate-400 uppercase font-semibold">Sharpe Ratio</p>
          <h2 className="text-3xl font-extrabold text-purple-400 mt-1">1.84</h2>
          <span className="text-xs text-purple-300">Annualized Risk-Adjusted</span>
        </div>
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <p className="text-xs text-slate-400 uppercase font-semibold">Cumulative LLM Cost</p>
          <h2 className="text-3xl font-extrabold text-amber-400 mt-1">$0.18</h2>
          <span className="text-xs text-slate-500">Redis Cache Hit: 78.4%</span>
        </div>
      </div>

      {/* Chart Section */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 mb-8">
        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <TrendingUp className="text-emerald-400" size={20} /> Real-time Equity Curve
        </h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={mockEquityData}>
              <XAxis dataKey="time" stroke="#64748b" />
              <YAxis domain={['auto', 'auto']} stroke="#64748b" />
              <Tooltip contentStyle={{ backgroundColor: "#0f172a", border: "1px solid #334155" }} />
              <Line type="monotone" dataKey="equity" stroke="#10b981" strokeWidth={2} dot={{ r: 4 }} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Recent Trades Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <AlertCircle className="text-blue-400" size={20} /> Recent Agent Executions & LLM Reasoning
        </h3>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-800 text-slate-400 uppercase text-xs">
              <tr>
                <th className="pb-3">Time</th>
                <th className="pb-3">Symbol</th>
                <th className="pb-3">Action</th>
                <th className="pb-3">Price</th>
                <th className="pb-3">Qty</th>
                <th className="pb-3">PnL</th>
                <th className="pb-3">Reasoning</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {trades.map((t) => (
                <tr key={t.id} className="hover:bg-slate-800/30">
                  <td className="py-3 font-mono text-slate-400">{t.time}</td>
                  <td className="py-3 font-bold">{t.symbol}</td>
                  <td className="py-3">
                    <span className={`px-2 py-0.5 rounded text-xs font-bold ${t.action === 'BUY' ? 'bg-emerald-950 text-emerald-400 border border-emerald-800' : 'bg-red-950 text-red-400 border border-red-800'}`}>
                      {t.action}
                    </span>
                  </td>
                  <td className="py-3 font-mono">${t.price.toFixed(2)}</td>
                  <td className="py-3 font-mono">{t.qty}</td>
                  <td className={`py-3 font-mono font-bold ${t.pnl >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                    {t.pnl !== 0 ? `+$${t.pnl.toFixed(2)}` : "-"}
                  </td>
                  <td className="py-3 text-slate-300 max-w-xs truncate">{t.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
