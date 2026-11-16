Fixtures recorded from the public APIs/sites on 2026-11-15/16 and trimmed
(long text fields truncated) or hand-edited where noted:

- `greenhouse_jobs.json`: boards-api.greenhouse.io/v1/boards/gitlab/jobs?content=true
- `lever_postings.json`: api.lever.co/v0/postings/spotify?mode=json
- `ashby_job_board.json`: api.ashbyhq.com/posting-api/job-board/linear
  (hand-edited: job 3 set to `isListed: false`, job 2 given a secondary
  location, to exercise those code paths)
- `personio.xml`: personio.jobs.personio.de/xml (recorded, then two
  hand-edited additions: description blocks on position 1 and a third
  position with a remote office, to exercise those code paths)
- `workable_jobs.json`: apply.workable.com/api/v1/widget/accounts/huggingface?details=true
  (first three jobs kept; a second location added to job 1 by hand to
  exercise multi-location grouping)
- `rss_feed.xml`: weworkremotely.com/remote-jobs.rss (first three items,
  descriptions replaced with a short excerpt). Keeps the non-standard
  `<region>` element; the registered source maps `location_field: region`
  and the fixture test also checks the `Company: Role` titles
- `atom_feed.xml`: go.dev/blog/feed.atom (first three entries, content
  replaced with a short excerpt)
- `teamtailor_feed.xml`: careers.epicompany.eu/jobs.rss (first two items,
  descriptions replaced with a short excerpt; the nested
  `tt:locations/tt:location/tt:name` shape is kept)
- `homerun_feed.xml`: feed.homerun.co/ghost (first two entries,
  descriptions replaced with a short excerpt; keeps the non-standard
  Atom `<description>` element)
- `html_listing.html`: hand-written listing page (there is no public
  fixture API) covering relative/absolute links, optional selectors and
  a missing link that must be skipped
