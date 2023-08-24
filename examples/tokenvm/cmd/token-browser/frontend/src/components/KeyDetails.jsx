import {useEffect, useState} from "react";
import {Avatar, Card, Divider, List, Spin, Timeline, Typography} from "antd";
import {GetMoreInformationFromURL} from "../../../wailsjs/go/main/App";

const UserGrid = ({users}) => (<List
    grid={{gutter: 16, column: 4}}
    dataSource={users}
    renderItem={(item, index) => (<List.Item key={index} style={{marginTop: "5px"}}>
        <Card.Meta
            avatar={<Avatar src={item.avatar_url}/>}
            title={item.login}
        />
    </List.Item>)}
/>);

const RepositoryDetails = ({repository, token = ""}) => {
    const [commits, setCommits] = useState([]);
    const [contributors, setContributors] = useState([]);
    const [stargazers, setStargazers] = useState([]);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        const getRepositoryDetails = async () => {
            setIsLoading(true);
            const stargazers = await GetMoreInformationFromURL(repository.stargazers_url, token);
            const commits = await GetMoreInformationFromURL(repository.commits_url.replace(/{\/[a-z]*}/, ""), token);
            const contributors = await GetMoreInformationFromURL(repository.contributors_url, token);
            setCommits(commits);
            setContributors(contributors);
            setStargazers(stargazers);
            setIsLoading(false);
        };
        getRepositoryDetails();
    }, [repository]);

    return (<Card
        title={repository.name}
        bordered={false}
        style={{
            margin: "1%",
        }}
    >
        {repository.description}
        <Divider/>
        <Spin tip="Loading" spinning={isLoading}>
            <Typography.Title level={5} style={{margin: 10}}>
                Contributors
            </Typography.Title>
            <UserGrid users={contributors}/>
            <Divider/>
            <Typography.Title level={5} style={{marginBottom: 15}}>
                Stargazers
            </Typography.Title>
            <UserGrid users={stargazers}/>
            <Divider/>
            <Typography.Title level={5} style={{marginBottom: 15}}>
                Commits
            </Typography.Title>
            <Timeline mode="alternate">
                {
                    commits.map((commit, index) => (
                        <Timeline.Item key={index}>{commit.commit?.message}</Timeline.Item>)
                    )
                }
            </Timeline>
        </Spin>
    </Card>);
};

export default KeyDetails;
