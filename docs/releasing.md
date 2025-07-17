# How to Create a Release for Cluster API Addon Provider for Cdk8s

This document describes the process for creating a new release of the Cluster API Addon Provider for Cdk8s. This process requires maintainer access to the repository.

1. **Check out code at the correct commit.**

    Currently this project releases from the `main` branch. Make sure your local copy of the code is up to date:

    ```shell
    git checkout main
    git fetch upstream
    git rebase upstream/main
    ```

2. **Choose the next semantic version.**

    Choose the next patch or minor version by incrementing the latest tag:

    ```shell
    git describe --tags --abbrev=0
    export RELEASE_VERSION=v0.2.1  # Replace "v0.2.1" with the actual next version
    ```

3. **Tag the code and push it upstream.**

    ```shell
    git push upstream main
    git tag -a $RELEASE_VERSION -m $RELEASE_VERSION
    git push upstream $RELEASE_VERSION
    ```

4. **Wait for a tagged image in the staging repository.**

    Pushing the new tag will trigger a [testgrid job](https://testgrid.k8s.io/sig-cluster-lifecycle-image-pushes#post-cluster-api-addon-provider-cdk8s-push-images) to build and push an image to the [staging repository](https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-cdk8s?project=k8s-staging-cluster-api-cdk8s). Wait for the job to complete and for the tagged image to be available before proceeding.

5. **Promote the release image.**

    Run the `make promote-images` command to promote the image from the staging repository to the production repository. If your git remotes don't use `https://` URLs, set the `USER_FORK` environment variable to your GitHub username.

    ```shell
    USER_FORK=<username> make promote-images
    ```

    This command will create a Pull Request at [k8s.io](https://github.com/kubernetes/k8s.io/pulls) to promote the image to production. Double-check that the SHA added by the PR matches the SHA of the staging image.

    See an [example PR](https://github.com/kubernetes/k8s.io/pull/6652).

6. **Wait for the release image to be available.**

    After the PR to promote the image has been approved and merged, wait for the image to be available:

    ```shell
    docker pull registry.k8s.io/cluster-api-cdk8s/cluster-api-cdk8s-controller:${RELEASE_VERSION}
    ```

7. **Update and publish the release on GitHub.**

    Pushing the new tag also triggered a [GitHub Action](https://github.com/eitco/cluster-api-addon-provider-cdk8s/actions/workflows/release.yml) which creates [a draft release](https://github.com/eitco/cluster-api-addon-provider-cdk8s/releases).

    Edit the draft release and click the "Generate release notes" button. This will populate the release notes with the PRs merged since the last release.

    If they contain any merge commits authored by a bot, click through to the original PR, then update the reference so that the actual author is credited.

    Once you are satisfied with your changes, publish the release so it's no longer a draft.

8. **Publicize the release.**

    Announce the new release on the [CAPI Slack channel](https://kubernetes.slack.com/archives/C8TSNPY4T).
