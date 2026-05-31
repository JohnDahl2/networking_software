# Network Engineer Questions

## Infrastructure / Deployment
- Is this running on a dedicated server, a VM, or someone's laptop?
- Does it need to run 24/7 or only when someone kicks off an extraction?
- Is Postgres running on the same machine or a separate server?

## File Delivery
- How do pcap files get to the machine this runs on? Manually copied, network share, SCP, something else?
- Are files coming from one location or multiple sensors across the data center?
- How big are typical pcap files in practice — MB or GB range?
- Should files be deleted or archived after extraction, or left alone?

## Data Volume
- How many pcap files would a typical extraction job involve?
- How frequently are new files generated — constantly, hourly, daily?
- Is there a retention concern — does old data need to be purged from Postgres eventually?

## Usage Patterns
- Is this one person using it or a small team?
- Does he need to trigger extractions manually or should it just run automatically when new files appear?
- What analytics does he actually want to run — is it ad hoc querying, specific dashboards, alerts?

## The Wireshark Replacement Question
- What specifically does he hate about Wireshark — the UI, the filtering, the performance on large files, the lack of aggregation?
- That answer will tell you what the frontend needs to prioritize.

---

## Answers
*(fill in below after the conversation)*

