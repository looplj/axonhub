# TTFT Latency Regression - Work Plan

## Objective
Diagnose and fix TTFT (Time To First Token) visibility issues in the latency column on the /project/requests page.

## Tasks

- [ ] Investigate current TTFT measurement implementation and data flow
- [ ] Identify root cause of TTFT visibility regression or missing data
- [ ] Implement fix for TTFT tracking or display logic
- [ ] Verify TTFT values appear correctly in latency column
- [ ] Test end-to-end request flow to confirm TTFT is captured and displayed

## Notes
- Focus on backend measurement and frontend display components
- Ensure TTFT is properly calculated and passed through the request lifecycle
- Verify database schema supports TTFT storage if needed