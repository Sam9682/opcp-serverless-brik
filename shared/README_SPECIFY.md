# SPECIFY an AI Context Feature

## Overview
The "SPECIFY an AI context" feature helps users transform brief, informal requests into detailed, actionable specifications that can be used with the "MODIFY the Application" option.

## Purpose
When users want to modify an application but aren't sure how to write a comprehensive specification, this feature acts as an AI assistant that:
- Analyzes brief user descriptions (1-2 sentences)
- Generates detailed technical specifications
- Provides structured, copy-paste ready context for the MODIFY action

## How to Use

### Step 1: Select the SPECIFY Option
1. Open the dashboard
2. In the "My Virtual Agents" panel, select your application
3. Choose "📝 SPECIFY an AI context" from the action dropdown

### Step 2: Provide a Brief Description
Enter a simple description of what you want to change, for example:
- "Add a contact form to the homepage"
- "Change the login button color to blue"
- "Fix the bug where users can't upload images"

### Step 3: Get the Specification
The AI will generate a comprehensive specification including:
- Clear objectives
- Technical requirements
- Implementation details
- Acceptance criteria
- Constraints and considerations

### Step 4: Use with MODIFY
1. Copy the generated specification
2. Change the action to "👨💻 MODIFY the App"
3. Paste the specification
4. Send to execute the modification

## Benefits
- **Saves Time**: No need to write detailed specs manually
- **Reduces Errors**: AI ensures all important aspects are covered
- **Better Results**: Detailed specs lead to more accurate implementations
- **Learning Tool**: See how brief ideas translate to technical requirements

## Context File
The feature uses `shared/SPECIFY_context.md` which contains the AI prompt template for generating specifications.

## Language Support
Available in both English and French through the translation system.
