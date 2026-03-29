package linear

const issueFragment = `
	id
	identifier
	title
	description
	priority
	state {
		id
		name
		color
		type
		position
	}
	assignee {
		id
		name
		email
	}
	labels {
		nodes {
			id
			name
			color
		}
	}
	createdAt
	updatedAt
`

const queryViewer = `query {
	viewer {
		id
		name
		email
	}
}`

const queryTeams = `query {
	teams {
		nodes {
			id
			name
			key
		}
	}
}`

const queryIssues = `query($teamId: String!, $first: Int!, $after: String, $filter: IssueFilter) {
	team(id: $teamId) {
		issues(first: $first, after: $after, filter: $filter) {
			nodes {` + issueFragment + `
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}
}`

const queryIssue = `query($id: String!) {
	issue(id: $id) {` + issueFragment + `
	}
}`

const queryWorkflowStates = `query($teamId: String!) {
	team(id: $teamId) {
		states {
			nodes {
				id
				name
				color
				type
				position
			}
		}
	}
}`
