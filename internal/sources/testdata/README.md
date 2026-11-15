Fixtures recorded from the public APIs on 2026-11-15 and trimmed to three
postings each (long text fields truncated):

- `greenhouse_jobs.json`: boards-api.greenhouse.io/v1/boards/gitlab/jobs?content=true
- `lever_postings.json`: api.lever.co/v0/postings/spotify?mode=json
- `ashby_job_board.json`: api.ashbyhq.com/posting-api/job-board/linear
  (hand-edited: job 3 set to `isListed: false`, job 2 given a secondary
  location, to exercise those code paths)
