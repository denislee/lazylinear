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
	project {
		id
		name
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

const queryTeamMetadata = `query($teamId: String!) {
	team(id: $teamId) {
		members(first: 100) {
			nodes {
				id
				name
				email
			}
		}
		cycles(first: 100) {
			nodes {
				id
				number
				name
				startsAt
				endsAt
				completedAt
			}
		}
	}
}`

const queryProjects = `query($after: String) {
	projects(first: 250, after: $after, includeArchived: true) {
		nodes {
			id
			name
			status {
				name
			}
			lead {
				id
			}
		}
		pageInfo {
			hasNextPage
			endCursor
		}
	}
}`
